package internalgrpc

import (
	"context"
	"errors"
	"fmt"
	"net"

	"github.com/mmeshcher/url-shortener/internal/grpc/proto"
	"github.com/mmeshcher/url-shortener/internal/middleware"
	"github.com/mmeshcher/url-shortener/internal/service"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

type ShortenerServer struct {
	proto.UnimplementedShortenerServiceServer
	service *service.ShortenerService
	logger  *zap.Logger
}

func NewShortenerServer(s *service.ShortenerService, logger *zap.Logger) *ShortenerServer {
	return &ShortenerServer{
		service: s,
		logger:  logger,
	}
}

func (s *ShortenerServer) ShortenURL(ctx context.Context, req *proto.URLShortenRequest) (*proto.URLShortenResponse, error) {
	userID, ok := middleware.GetUserIDFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "user id not found in context")
	}

	shortURL, err := s.service.CreateShortURL(ctx, req.Url, userID)
	if err != nil {
		if errors.Is(err, service.ErrURLAlreadyExists) {
			return &proto.URLShortenResponse{Result: shortURL}, nil
		}
		s.logger.Error("Failed to shorten URL", zap.Error(err))
		return nil, status.Errorf(codes.Internal, "failed to shorten URL: %v", err)
	}

	return &proto.URLShortenResponse{Result: shortURL}, nil
}

func (s *ShortenerServer) ExpandURL(ctx context.Context, req *proto.URLExpandRequest) (*proto.URLExpandResponse, error) {
	originalURL, exists, deleted := s.service.GetOriginalURL(req.Id)
	if !exists {
		return nil, status.Error(codes.NotFound, "URL not found")
	}
	if deleted {
		return nil, status.Error(codes.Unavailable, "URL deleted")
	}

	return &proto.URLExpandResponse{Result: originalURL}, nil
}

func (s *ShortenerServer) ListUserURLs(ctx context.Context, _ *emptypb.Empty) (*proto.UserURLsResponse, error) {
	userID, ok := middleware.GetUserIDFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "user id not found in context")
	}

	urls, err := s.service.GetUserURLs(ctx, userID)
	if err != nil {
		s.logger.Error("Failed to get user URLs", zap.Error(err))
		return nil, status.Errorf(codes.Internal, "failed to get user URLs: %v", err)
	}

	if len(urls) == 0 {
		return &proto.UserURLsResponse{}, nil
	}

	data := make([]*proto.URLData, len(urls))
	for i, u := range urls {
		data[i] = &proto.URLData{
			ShortUrl:    u.ShortURL,
			OriginalUrl: u.OriginalURL,
		}
	}

	return &proto.UserURLsResponse{Url: data}, nil
}

// AuthInterceptor extracts user ID from metadata or creates a new one.
func AuthInterceptor(auth *middleware.AuthMiddleware, logger *zap.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "metadata is missing")
		}

		values := md.Get("authorization")
		var userID string

		if len(values) > 0 {
			// Try to parse existing token
			userID, _ = auth.ParseToken(values[0])
			if userID == "" {
				// If token is invalid, we could either return error or create new user
				// Based on HTTP logic, we usually create a new user if cookie is missing/invalid
				userID = auth.CreateNewUserID()
			}
		} else {
			userID = auth.CreateNewUserID()
		}

		ctx = context.WithValue(ctx, middleware.UserIDKey(), userID)
		return handler(ctx, req)
	}
}

func RegisterShortenerServer(s *grpc.Server, server *ShortenerServer) {
	proto.RegisterShortenerServiceServer(s, server)
}

func StartGRPCServer(addr string, s *ShortenerServer, auth *middleware.AuthMiddleware, logger *zap.Logger) error {
	listen, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	server := grpc.NewServer(
		grpc.UnaryInterceptor(AuthInterceptor(auth, logger)),
	)
	proto.RegisterShortenerServiceServer(server, s)

	logger.Info("gRPC server starting", zap.String("address", addr))
	if err := server.Serve(listen); err != nil {
		return fmt.Errorf("failed to serve: %w", err)
	}

	return nil
}
