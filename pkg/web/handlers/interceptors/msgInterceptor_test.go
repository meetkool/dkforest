package interceptors

import (
	"context"
	"github.com/grpc-ecosystem/go-grpc-middleware"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

type zapLogger struct {
	logger *zap.Logger
}

func NewZapLogger(logger *zap.Logger) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		startTime := time.Now()
		resp, err := handler(ctx, req)
		duration := time.Since(startTime)

		if err != nil {
			level := zap.ErrorLevel
			if grpc.Code(err) == codes.Canceled {
				level = zap.WarnLevel
			}
			zapRecord := zap.With(
				zap.String("method", info.FullMethod),
				zap.Duration("duration", duration),
				zap.Error(err),
			)
			zap.L().Check(level, "gRPC request failed").Write(zapRecord)
		} else {
			zapRecord := zap.With(
				zap.String("method", info.FullMethod),
				zap.Duration("duration", duration),
			)
			zap.L().Info("gRPC request succeeded").Write(zapRecord)
		}

		return resp, err
	}
}

func NewInterceptor(logger *zap.Logger) grpc.UnaryServerInterceptor {
	return grpc_middleware.ChainUnaryServer(
		NewZapLogger(logger),
	)

