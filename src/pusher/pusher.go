package pusher

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Pusher struct {
	url        string
	job        string
	instance   string
	httpClient *http.Client
}

type Option func(*Pusher)

func WithJob(job string) Option {
	return func(p *Pusher) {
		p.job = job
	}
}

func WithInstance(instance string) Option {
	return func(p *Pusher) {
		p.instance = instance
	}
}

func WithTimeout(timeout time.Duration) Option {
	return func(p *Pusher) {
		p.httpClient.Timeout = timeout
	}
}

func NewPusher(url string, opts ...Option) *Pusher {
	p := &Pusher{
		url: url,
		job: "node",
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}

	for _, opt := range opts {
		opt(p)
	}

	return p
}
func (p *Pusher) Push(metrics []byte) error {
	req, err := http.NewRequest(http.MethodPost, p.pushURL(), bytes.NewReader(metrics))
	if err != nil {
		return fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("Content-Type", "text/plain; version=0.0.4")
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("推送指标失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("推送失败，状态码%d: %s", resp.StatusCode, string(body))
	}

	return nil
}

func (p *Pusher) pushURL() string {
	url := p.url + "/metrics/job/" + p.job
	if p.instance != "" {
		url += "/instance/" + p.instance
	}
	return url
}
