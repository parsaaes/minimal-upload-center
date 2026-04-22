package main

import (
	"embed"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/sirupsen/logrus"
	"github.com/tus/tusd/v2/pkg/filelocker"
	"github.com/tus/tusd/v2/pkg/filestore"
	tusd "github.com/tus/tusd/v2/pkg/handler"
)

//go:embed front
var frontFS embed.FS

type tusInfo struct {
	ID       string            `json:"ID"`
	Size     int64             `json:"Size"`
	Offset   int64             `json:"Offset"`
	MetaData map[string]string `json:"MetaData"`
}

type fileEntry struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Size     int64  `json:"size"`
	FileType string `json:"fileType"`
	Modified int64  `json:"modified"`
}

const uploadPath = "./uploads"

func main() {
	store := filestore.New(uploadPath)
	locker := filelocker.New(uploadPath)

	composer := tusd.NewStoreComposer()
	store.UseIn(composer)
	locker.UseIn(composer)

	handler, err := tusd.NewHandler(tusd.Config{
		BasePath:              "/files/",
		StoreComposer:         composer,
		NotifyCompleteUploads: true,
	})
	if err != nil {
		logrus.Fatalf("unable to create handler: %s", err)
	}

	go func() {
		for {
			event := <-handler.CompleteUploads

			logrus.Infof("Upload %s finished", event.Upload.ID)
		}
	}()

	e := echo.New()

	defer func() {
		logrus.Info(e.Close())
	}()

	e.Use(middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		LogMethod:  true,
		LogURI:     true,
		LogStatus:  true,
		LogLatency: true,
		LogError:   true,
		LogValuesFunc: func(c echo.Context, v middleware.RequestLoggerValues) error {
			logrus.WithError(v.Error).WithFields(logrus.Fields{
				"method":     v.Method,
				"uri":        v.URI,
				"status":     v.Status,
				"latency":    v.Latency,
				"remote_ip":  c.RealIP(),
				"user_agent": c.Request().UserAgent(),
			}).Info("handled request")

			return nil
		},
	}))

	e.GET("/api/files", listFilesHandler)
	e.GET("/healthz", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	tusWrapper := http.StripPrefix("/files", handler)
	e.Any("/files/*", echo.WrapHandler(tusWrapper))
	e.Any("/files", echo.WrapHandler(tusWrapper))

	e.StaticFS("/", echo.MustSubFS(frontFS, "front"))

	e.HideBanner = true
	e.HidePort = true

	logrus.Infof("server started on :7677")

	if err := e.Start(":7677"); err != nil {
		logrus.Fatalf("server error: %s", err)
	}
}

func listFilesHandler(c echo.Context) error {
	entries, err := os.ReadDir(uploadPath)
	if err != nil {
		logrus.Errorf("unable to list files: %s", err)

		return c.String(http.StatusInternalServerError, "could not read files")
	}

	files := make([]fileEntry, 0, len(entries))

	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".info") {
			continue
		}

		data, err := os.ReadFile(filepath.Join(uploadPath, entry.Name()))
		if err != nil {
			logrus.WithError(err).Warnf("unable to read file %s", entry.Name())

			continue
		}

		var info tusInfo
		if err := json.Unmarshal(data, &info); err != nil {
			logrus.WithError(err).Warnf("unable to parse file info %s", entry.Name())

			continue
		}

		filePath := filepath.Join(uploadPath, strings.TrimSuffix(entry.Name(), ".info"))
		stat, err := os.Stat(filePath)
		if err != nil {
			logrus.WithError(err).Warnf("unable to stat file %s", entry.Name())

			continue
		}

		if stat.Size() != info.Size {
			logrus.Warnf("skipping partially uploaded file %s (%d/%d)", entry.Name(), stat.Size(), info.Size)

			continue
		}

		files = append(files, fileEntry{
			ID:       info.ID,
			Name:     info.MetaData["filename"],
			Size:     info.Size,
			FileType: info.MetaData["filetype"],
			Modified: stat.ModTime().Unix(),
		})
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].Modified > files[j].Modified
	})

	return c.JSON(http.StatusOK, files)
}
