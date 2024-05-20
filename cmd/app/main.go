package main

import (
	"context"
	"github.com/hashicorp/go-hclog"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"grpc/internal/interceptor"
	"grpc/internal/metric"
	"grpc/internal/server"
	protos "grpc/pkg/product_v1"
	"log"
	"net"
	"net/http"
	"os"
)

func main() {
	ctx := context.Background()
	err := metric.Init(ctx)
	mongoURI := "mongodb://localhost:27017"
	log := hclog.Default()
	gs := grpc.NewServer(
		grpc.UnaryInterceptor(
			interceptor.MetricsInterceptor,
		),
	)
	reflection.Register(gs)
	pc, err := server.NewProductServer(log, mongoURI)
	protos.RegisterProductV1Server(gs, pc)
	l, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Error("Failed to listen", err)
		os.Exit(1)
	}
	go func() {
		err = runPrometheus()
		if err != nil {
			log.Error("Failed to run prometheus", err)
			os.Exit(1)
		}
	}()
	if err = gs.Serve(l); err != nil {
		log.Error("Failed to serve", "err", err)
		os.Exit(1)
	}
}
func runPrometheus() error {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	prometheusServer := &http.Server{
		Addr:    "localhost:2112",
		Handler: mux,
	}
	log.Printf("Prometheus server is running on %s", "localhost:2112")
	err := prometheusServer.ListenAndServe()
	if err != nil {
		return err
	}
	return nil
}
