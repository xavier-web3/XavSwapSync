package cmd

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/xavier-web3/XavSwapBase/logger/xzap"
	"github.com/xavier-web3/XavSwapSync/service"
	"github.com/xavier-web3/XavSwapSync/service/config"
	"go.uber.org/zap"
)

var DaemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "sync easy swap order info.",
	Long:  "sync easy swap order info.",
	Run: func(cmd *cobra.Command, args []string) {
		wg := &sync.WaitGroup{}
		wg.Add(1)
		ctx := context.Background()
		ctx, cancel := context.WithCancel(ctx)
		// rpc退出信号通知chan
		onSyncExit := make(chan error, 1)

		go func() {
			defer wg.Done()
			// 读取和解析配置文件
			cfg, err := config.UnmarshalCmdConfig()
			if err != nil {
				xzap.WithContext(ctx).Error("Failed to unmarshal config", zap.Error(err))
				onSyncExit <- err
				return
			}
			// 初始化日志模块
			_, err = xzap.SetUp(*cfg.Log)
			if err != nil {
				xzap.WithContext(ctx).Error("Failed to set up logger", zap.Error(err))
				onSyncExit <- err
				return
			}
			xzap.WithContext(ctx).Info("sync server start", zap.Any("config", cfg))
			// 初始化服务
			s, err := service.New(ctx, cfg)
			if err != nil {
				xzap.WithContext(ctx).Error("Failed to start sync server", zap.Error(err))
				onSyncExit <- err
				return
			}
			if err := s.Start(); err != nil { // 启动服务
				xzap.WithContext(ctx).Error("Failed to start sync server", zap.Error(err))
				onSyncExit <- err
				return
			}
			// 开启pprof，用于性能监控
			if cfg.Monitor.PprofEnable {
				http.ListenAndServe(fmt.Sprintf("0.0.0.0:%d", cfg.Monitor.PprofPort), nil)
			}
		}()
		// 信号通知chan
		onSignal := make(chan os.Signal)
		// 优雅退出
		signal.Notify(onSignal, syscall.SIGINT, syscall.SIGTERM)
		select {
		case sig := <-onSignal:
			switch sig {
			case syscall.SIGINT, syscall.SIGTERM:
				cancel()
				xzap.WithContext(ctx).Info("Exit by signal", zap.String("signal", sig.String()))
			}
		case err := <-onSyncExit:
			cancel()
			xzap.WithContext(ctx).Error("Exit by error", zap.Error(err))
		}
		wg.Wait()
	},
}

func init() {
	// 将api初始化命令添加到主命令中
	rootCmd.AddCommand(DaemonCmd)
}
