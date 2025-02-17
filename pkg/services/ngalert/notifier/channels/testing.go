package channels

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/grafana/grafana/pkg/bus"
	"github.com/grafana/grafana/pkg/infra/tracing"
	"github.com/grafana/grafana/pkg/models"
	"github.com/grafana/grafana/pkg/services/notifications"
	"github.com/grafana/grafana/pkg/setting"
)

type fakeImageStore struct {
	Images []*Image
}

// getImage returns an image with the same token.
func (f *fakeImageStore) GetImage(_ context.Context, token string) (*Image, error) {
	for _, img := range f.Images {
		if img.Token == token {
			return img, nil
		}
	}
	return nil, ErrImageNotFound
}

// newFakeImageStore returns an image store with N test images.
// Each image has a token and a URL, but does not have a file on disk.
func newFakeImageStore(n int) ImageStore {
	s := fakeImageStore{}
	for i := 1; i <= n; i++ {
		s.Images = append(s.Images, &Image{
			Token:     fmt.Sprintf("test-image-%d", i),
			URL:       fmt.Sprintf("https://www.example.com/test-image-%d.jpg", i),
			CreatedAt: time.Now().UTC(),
		})
	}
	return &s
}

// newFakeImageStoreWithFile returns an image store with N test images.
// Each image has a token, path and a URL, where the path is 1x1 transparent
// PNG on disk. The test should call deleteFunc to delete the images from disk
// at the end of the test.
// nolint:deadcode,unused
func newFakeImageStoreWithFile(t *testing.T, n int) ImageStore {
	var (
		files []string
		s     fakeImageStore
	)

	t.Cleanup(func() {
		// remove all files from disk
		for _, f := range files {
			if err := os.Remove(f); err != nil {
				t.Logf("failed to delete file: %s", err)
			}
		}
	})

	for i := 1; i <= n; i++ {
		file, err := newTestImage()
		if err != nil {
			t.Fatalf("failed to create test image: %s", err)
		}
		files = append(files, file)
		s.Images = append(s.Images, &Image{
			Token:     fmt.Sprintf("test-image-%d", i),
			Path:      file,
			URL:       fmt.Sprintf("https://www.example.com/test-image-%d", i),
			CreatedAt: time.Now().UTC(),
		})
	}

	return &s
}

// nolint:deadcode,unused
func newTestImage() (string, error) {
	f, err := os.CreateTemp("", "test-image-*.png")
	if err != nil {
		return "", fmt.Errorf("failed to create temp image: %s", err)
	}

	// 1x1 transparent PNG
	b, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNkYAAAAAYAAjCB0C8AAAAASUVORK5CYII=")
	if err != nil {
		return f.Name(), fmt.Errorf("failed to decode PNG data: %s", err)
	}

	if _, err := f.Write(b); err != nil {
		return f.Name(), fmt.Errorf("failed to write to file: %s", err)
	}

	if err := f.Close(); err != nil {
		return f.Name(), fmt.Errorf("failed to close file: %s", err)
	}

	return f.Name(), nil
}

// mockTimeNow replaces function timeNow to return constant time.
// It returns a function that resets the variable back to its original value.
// This allows usage of this function with defer:
//
//	func Test (t *testing.T) {
//	   now := time.Now()
//	   defer mockTimeNow(now)()
//	   ...
//	}
func mockTimeNow(constTime time.Time) func() {
	timeNow = func() time.Time {
		return constTime
	}
	return resetTimeNow
}

// resetTimeNow resets the global variable timeNow to the default value, which is time.Now
func resetTimeNow() {
	timeNow = time.Now
}

type notificationServiceMock struct {
	Webhook     SendWebhookSettings
	EmailSync   SendEmailSettings
	ShouldError error
}

func (ns *notificationServiceMock) SendWebhook(ctx context.Context, cmd *SendWebhookSettings) error {
	ns.Webhook = *cmd
	return ns.ShouldError
}
func (ns *notificationServiceMock) SendEmail(ctx context.Context, cmd *SendEmailSettings) error {
	ns.EmailSync = *cmd
	return ns.ShouldError
}

func mockNotificationService() *notificationServiceMock { return &notificationServiceMock{} }

type emailSender struct {
	ns *notifications.NotificationService
}

func (e emailSender) SendEmail(ctx context.Context, cmd *SendEmailSettings) error {
	attached := make([]*models.SendEmailAttachFile, 0, len(cmd.AttachedFiles))
	for _, file := range cmd.AttachedFiles {
		attached = append(attached, &models.SendEmailAttachFile{
			Name:    file.Name,
			Content: file.Content,
		})
	}
	return e.ns.SendEmailCommandHandlerSync(ctx, &models.SendEmailCommandSync{
		SendEmailCommand: models.SendEmailCommand{
			To:            cmd.To,
			SingleEmail:   cmd.SingleEmail,
			Template:      cmd.Template,
			Subject:       cmd.Subject,
			Data:          cmd.Data,
			Info:          cmd.Info,
			ReplyTo:       cmd.ReplyTo,
			EmbeddedFiles: cmd.EmbeddedFiles,
			AttachedFiles: attached,
		},
	})
}

func createEmailSender(t *testing.T) *emailSender {
	t.Helper()

	tracer := tracing.InitializeTracerForTest()
	bus := bus.ProvideBus(tracer)

	cfg := setting.NewCfg()
	cfg.StaticRootPath = "../../../../../public/"
	cfg.BuildVersion = "4.0.0"
	cfg.Smtp.Enabled = true
	cfg.Smtp.TemplatesPatterns = []string{"emails/*.html", "emails/*.txt"}
	cfg.Smtp.FromAddress = "from@address.com"
	cfg.Smtp.FromName = "Grafana Admin"
	cfg.Smtp.ContentTypes = []string{"text/html", "text/plain"}
	cfg.Smtp.Host = "localhost:1234"
	mailer := notifications.NewFakeMailer()

	ns, err := notifications.ProvideService(bus, cfg, mailer, nil)
	require.NoError(t, err)

	return &emailSender{ns: ns}
}
