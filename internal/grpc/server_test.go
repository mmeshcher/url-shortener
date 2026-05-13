package internalgrpc

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/mmeshcher/url-shortener/internal/grpc/proto"
	"github.com/mmeshcher/url-shortener/internal/middleware"
	"github.com/mmeshcher/url-shortener/internal/repository"
	"github.com/mmeshcher/url-shortener/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/emptypb"
)

func TestGRPCServer(t *testing.T) {
	logger := zap.NewNop()
	repo := repository.NewMemoryRepository("", logger)
	shortenerService := service.NewShortenerService("http://localhost:8080", repo, logger)
	auth := middleware.NewAuthMiddleware("secret", logger)

	// Start server on a random port
	listen, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	addr := listen.Addr().String()

	s := NewShortenerServer(shortenerService, logger)
	gSrv := grpc.NewServer(
		grpc.UnaryInterceptor(AuthInterceptor(auth, logger)),
	)
	proto.RegisterShortenerServiceServer(gSrv, s)

	go func() {
		if err := gSrv.Serve(listen); err != nil {
			return
		}
	}()
	defer gSrv.GracefulStop()

	conn, err := grpc.NewClient(
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	for {
		state := conn.GetState()
		if state == connectivity.Ready {
			break
		}
		if !conn.WaitForStateChange(ctx, state) {
			break
		}
	}

	client := proto.NewShortenerServiceClient(conn)

	t.Run("ShortenURL and ExpandURL", func(t *testing.T) {
		originalURL := "https://example.com"
		userID := "test-user"
		token := auth.SignUserID(userID)

		// Create context with metadata
		ctx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("authorization", token))

		// Shorten
		shortResp, err := client.ShortenURL(ctx, &proto.URLShortenRequest{Url: originalURL})
		require.NoError(t, err)
		assert.Contains(t, shortResp.Result, "http://localhost:8080/")

		shortID := shortResp.Result[len("http://localhost:8080/"):]

		// Expand
		expandResp, err := client.ExpandURL(ctx, &proto.URLExpandRequest{Id: shortID})
		require.NoError(t, err)
		assert.Equal(t, originalURL, expandResp.Result)
	})

	t.Run("ListUserURLs", func(t *testing.T) {
		userID := "test-user-2"
		token := auth.SignUserID(userID)
		ctx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("authorization", token))

		// Shorten two URLs
		_, err := client.ShortenURL(ctx, &proto.URLShortenRequest{Url: "https://yandex.ru"})
		require.NoError(t, err)
		_, err = client.ShortenURL(ctx, &proto.URLShortenRequest{Url: "https://google.com"})
		require.NoError(t, err)

		// List
		listResp, err := client.ListUserURLs(ctx, &emptypb.Empty{})
		require.NoError(t, err)
		assert.Len(t, listResp.Url, 2)
		assert.Equal(t, "https://yandex.ru", listResp.Url[0].OriginalUrl)
		assert.Equal(t, "https://google.com", listResp.Url[1].OriginalUrl)
	})

	t.Run("Unauthenticated", func(t *testing.T) {
		// No metadata
		_, err := client.ShortenURL(context.Background(), &proto.URLShortenRequest{Url: "https://example.com"})
		// Based on our interceptor, it creates a new user if metadata is missing
		assert.NoError(t, err)
	})
}
