package observer

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/chromedp/chromedp"
)

// Screenshot captures a browser screenshot using CDP.
type Screenshot struct{}

// CaptureBytes takes a screenshot of the given URL and returns raw PNG bytes.
func (s *Screenshot) CaptureBytes(url string) ([]byte, error) {
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var buf []byte
	if err := chromedp.Run(ctx,
		chromedp.Navigate(url),
		chromedp.WaitReady("body"),
		chromedp.CaptureScreenshot(&buf),
	); err != nil {
		return nil, fmt.Errorf("screenshot: %w", err)
	}

	return buf, nil
}

// CaptureToFile takes a screenshot and saves it to a file.
func (s *Screenshot) CaptureToFile(url, path string) error {
	buf, err := s.CaptureBytes(url)
	if err != nil {
		return err
	}
	return os.WriteFile(path, buf, 0o644)
}
