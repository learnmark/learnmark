package gateway

import (
	"context"
	"fmt"
	"mime"
	"net/http"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	v1 "github.com/learnmark/learnmark/api/learnmark/v1"
	"github.com/learnmark/learnmark/pkg/log"
	"github.com/learnmark/learnmark/pkg/utils"
	swaggerui "github.com/learnmark/learnmark/swagger-ui"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const staticPrefix = "/api/v1/swagger/"

func NewGateway(ctx context.Context, conn *grpc.ClientConn, opts []runtime.ServeMuxOption) (http.Handler, error) {

	mux := runtime.NewServeMux(opts...)

	for _, f := range []func(context.Context, *runtime.ServeMux, *grpc.ClientConn) error{
		v1.RegisterLearnmarkHandler,
	} {
		if err := f(ctx, mux, conn); err != nil {
			return nil, err
		}
	}
	return mux, nil
}

func Run(ctx context.Context, opts utils.Options) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	conn, err := grpc.NewClient(opts.GRPCAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}
	go func() {
		<-ctx.Done()
		if err := conn.Close(); err != nil {
			log.L(ctx).Errorf("Failed to close a client connection to the gRPC server: %v", err)
		}
	}()

	mux := http.NewServeMux()
	mux.HandleFunc("/openapiv2/", openAPIServer(opts.OpenAPIDir))
	mux.HandleFunc("/grpcHealthz", grpcHealthzServer(conn))
	mux.Handle("/sys/", runHealthCheck())
	mime.AddExtensionType(".svg", "image/svg+xml")

	mux.Handle(staticPrefix, http.StripPrefix(staticPrefix, http.FileServer(http.FS(swaggerui.Resources))))

	gw, err := NewGateway(ctx, conn, opts.Mux)
	if err != nil {
		return err
	}
	mux.Handle("/", gw)

	s := &http.Server{
		Addr:    opts.HTTPAddr,
		Handler: allowCORS(mux),
	}
	go func() {
		<-ctx.Done()
		log.L(ctx).Infof("Shutting down the http server")
		if err := s.Shutdown(context.Background()); err != nil {
			log.L(ctx).Errorf("Failed to shutdown http server: %v", err)
		}
	}()

	log.L(ctx).Infof("Starting listening at: %s", opts.HTTPAddr)
	if err := s.ListenAndServe(); err != http.ErrServerClosed {
		log.L(ctx).Errorf("Failed to listen and serve: %v", err)
		return fmt.Errorf("error: %w", err)
	}
	return nil
}
