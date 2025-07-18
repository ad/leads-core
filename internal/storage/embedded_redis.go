package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/ad/leads-core/pkg/logger"
	"github.com/nalgeon/redka"
	"github.com/nalgeon/redka/redsrv"
	_ "modernc.org/sqlite"
)

// EmbeddedRedisServer управляет встроенным Redis сервером на основе Redka
type EmbeddedRedisServer struct {
	db     *redka.DB
	srv    *redsrv.Server
	addr   string
	dbPath string
}

// NewEmbeddedRedisServer создает новый встроенный Redis сервер
func NewEmbeddedRedisServer(port, dbPath string) (*EmbeddedRedisServer, error) {
	addr := ":" + port

	logger.Info("Creating embedded Redis server", map[string]interface{}{
		"addr":    addr,
		"db_path": dbPath,
	})

	opts := redka.Options{
		DriverName: "sqlite",
	}

	// Открываем базу данных Redka
	db, err := redka.Open(dbPath, &opts)
	if err != nil {
		return nil, fmt.Errorf("failed to open Redka database: %w", err)
	}

	// Создаем сервер
	srv := redsrv.New("tcp", addr, db)

	return &EmbeddedRedisServer{
		db:     db,
		srv:    srv,
		addr:   addr,
		dbPath: dbPath,
	}, nil
}

// Start запускает встроенный Redis сервер
func (e *EmbeddedRedisServer) Start() error {
	logger.Info("Starting embedded Redis server", map[string]interface{}{
		"addr": e.addr,
	})

	// Создаем канал для получения уведомления о готовности сервера
	ready := make(chan error, 1)

	// Запускаем сервер в горутине
	go func() {
		if err := e.srv.Start(ready); err != nil {
			logger.Error("Embedded Redis server error", map[string]interface{}{
				"error": err.Error(),
			})
			ready <- err
			return
		}
	}()

	// Ждем готовности сервера с таймаутом
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	select {
	case err := <-ready:
		if err != nil {
			return fmt.Errorf("failed to start embedded Redis server: %w", err)
		}
		logger.Info("Embedded Redis server started successfully", map[string]interface{}{
			"addr": e.addr,
		})
		return nil
	case <-ctx.Done():
		return fmt.Errorf("timeout waiting for embedded Redis server to start")
	}
}

// Stop останавливает встроенный Redis сервер
func (e *EmbeddedRedisServer) Stop() error {
	logger.Info("Stopping embedded Redis server", map[string]interface{}{
		"addr": e.addr,
	})

	// Останавливаем сервер с таймаутом
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- e.srv.Stop()
	}()

	select {
	case err := <-done:
		if err != nil {
			logger.Error("Error stopping embedded Redis server", map[string]interface{}{
				"error": err.Error(),
			})
		} else {
			logger.Info("Embedded Redis server stopped successfully")
		}

		// Закрываем базу данных
		if closeErr := e.db.Close(); closeErr != nil {
			logger.Error("Error closing Redka database", map[string]interface{}{
				"error": closeErr.Error(),
			})
			if err == nil {
				err = closeErr
			}
		}

		return err
	case <-ctx.Done():
		logger.Error("Timeout stopping embedded Redis server")
		return fmt.Errorf("timeout stopping embedded Redis server")
	}
}

// GetAddr возвращает адрес сервера
func (e *EmbeddedRedisServer) GetAddr() string {
	return e.addr
}

// GetDB возвращает экземпляр базы данных Redka
func (e *EmbeddedRedisServer) GetDB() *redka.DB {
	return e.db
}
