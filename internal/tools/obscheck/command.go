package obscheck

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/sandeepkv93/secure-observable-go-backend-starter-kit/internal/tools/common"
	"github.com/sandeepkv93/secure-observable-go-backend-starter-kit/internal/tools/loadgen"
	"github.com/sandeepkv93/secure-observable-go-backend-starter-kit/internal/tools/ui"
)

type options struct {
	grafanaURL      string
	grafanaUser     string
	grafanaPassword string
	serviceName     string
	window          time.Duration
	ci              bool
	baseURL         string
}

func NewRootCommand() *cobra.Command {
	opts := &options{}
	cmd := &cobra.Command{Use: "obscheck", Short: "Verify metrics, traces and logs correlation"}
	cmd.PersistentFlags().StringVar(&opts.grafanaURL, "grafana-url", "http://localhost:3000", "Grafana base URL")
	cmd.PersistentFlags().StringVar(&opts.grafanaUser, "grafana-user", "admin", "Grafana username")
	cmd.PersistentFlags().StringVar(&opts.grafanaPassword, "grafana-password", "admin", "Grafana password")
	cmd.PersistentFlags().StringVar(&opts.serviceName, "service-name", "secure-observable-go-backend-starter-kit", "OTel service name")
	cmd.PersistentFlags().DurationVar(&opts.window, "window", 20*time.Minute, "query lookback window")
	cmd.PersistentFlags().BoolVar(&opts.ci, "ci", false, "non-interactive machine-readable output")
	cmd.PersistentFlags().StringVar(&opts.baseURL, "base-url", "http://localhost:8080", "API base URL for traffic")
	cmd.AddCommand(newRunCommand(opts))
	return cmd
}

func newRunCommand(opts *options) *cobra.Command {
	return &cobra.Command{
		Use:   "run",
		Short: "Generate traffic and validate exemplar->trace->log path",
		RunE: func(cmd *cobra.Command, args []string) error {
			details, err := run(opts, "obscheck run", func(ctx context.Context) ([]string, error) {
				lgRes, err := loadgen.Run(ctx, loadgen.Config{
					BaseURL:     opts.baseURL,
					Profile:     "mixed",
					Duration:    6 * time.Second,
					RPS:         20,
					Concurrency: 6,
					Seed:        42,
				})
				if err != nil {
					return nil, err
				}
				details := []string{fmt.Sprintf("traffic generated total=%d failures=%d", lgRes.TotalRequests, lgRes.Failures)}
				recentCutoff := time.Now().Add(-2 * time.Minute)
				time.Sleep(8 * time.Second)

				traceID, err := fetchTraceIDFromExemplar(ctx, *opts, recentCutoff)
				if err != nil {
					return details, err
				}
				details = append(details, "exemplar trace_id="+traceID)

				if err := verifyTempoTrace(ctx, *opts, traceID); err != nil {
					return details, err
				}
				details = append(details, "tempo trace lookup: ok")

				if err := verifyLokiTraceLogs(ctx, *opts, traceID); err != nil {
					return details, err
				}
				details = append(details, "loki trace correlation: ok")
				return details, nil
			})
			if opts.ci {
				common.PrintCIResult(err == nil, "obscheck run", details, err)
			}
			if err != nil {
				os.Exit(4)
			}
			return nil
		},
	}
}

func run(opts *options, title string, fn func(context.Context) ([]string, error)) ([]string, error) {
	if opts.ci {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()
		return fn(ctx)
	}
	return ui.Run(title, fn)
}

func grafanaGET(ctx context.Context, opts options, path string) ([]byte, error) {
	u, err := url.Parse(opts.grafanaURL)
	if err != nil {
		return nil, err
	}
	rel, err := url.Parse(path)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.ResolveReference(rel).String(), nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(opts.grafanaUser, opts.grafanaPassword)
	resp, err := (&http.Client{Timeout: 20 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("grafana request failed: %s", resp.Status)
	}
	return ioReadAll(resp.Body)
}

func fetchTraceIDFromExemplar(ctx context.Context, opts options, notBefore time.Time) (string, error) {
	start := time.Now().Add(-opts.window).Unix()
	end := time.Now().Unix()
	path := fmt.Sprintf("/api/datasources/proxy/uid/mimir/api/v1/query_exemplars?query=auth_request_duration_seconds_bucket&start=%d&end=%d", start, end)
	body, err := grafanaGET(ctx, opts, path)
	if err != nil {
		return "", err
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", err
	}
	data, _ := payload["data"].([]any)
	var bestTraceID string
	var bestTS float64
	for _, series := range data {
		sm, _ := series.(map[string]any)
		exemplars, _ := sm["exemplars"].([]any)
		for _, e := range exemplars {
			em, _ := e.(map[string]any)
			labels, _ := em["labels"].(map[string]any)
			timestamp, _ := em["timestamp"].(float64)
			if timestamp <= 0 {
				continue
			}
			if notBefore.Unix() > 0 && int64(timestamp) < notBefore.Unix() {
				continue
			}
			if tid, ok := labels["trace_id"].(string); ok && len(tid) == 32 && timestamp > bestTS {
				bestTS = timestamp
				bestTraceID = tid
			}
		}
	}
	if bestTraceID != "" {
		return bestTraceID, nil
	}
	return "", fmt.Errorf("no recent trace_id exemplar found")
}

func verifyTempoTrace(ctx context.Context, opts options, traceID string) error {
	path := fmt.Sprintf("/api/datasources/proxy/uid/tempo/api/traces/%s", traceID)
	var lastErr error
	for i := 0; i < 5; i++ {
		body, err := grafanaGET(ctx, opts, path)
		if err != nil {
			lastErr = err
			time.Sleep(2 * time.Second)
			continue
		}
		var payload map[string]any
		if err := json.Unmarshal(body, &payload); err != nil {
			return err
		}
		batches, _ := payload["batches"].([]any)
		if len(batches) > 0 {
			return nil
		}
		lastErr = fmt.Errorf("tempo trace has no batches yet")
		time.Sleep(2 * time.Second)
	}
	if lastErr != nil {
		return lastErr
	}
	return fmt.Errorf("tempo trace lookup failed")
}

func verifyLokiTraceLogs(ctx context.Context, opts options, traceID string) error {
	nowNS := time.Now().UnixNano()
	startNS := nowNS - int64(30*time.Minute)
	queries := []string{
		fmt.Sprintf("{service_name=\"%s\"} | json | trace_id=\"%s\"", opts.serviceName, traceID),
		fmt.Sprintf("{service_name=~\".+\"} | json | trace_id=\"%s\"", traceID),
	}
	for _, raw := range queries {
		q := url.QueryEscape(raw)
		path := fmt.Sprintf("/api/datasources/proxy/uid/loki/loki/api/v1/query_range?query=%s&start=%d&end=%d&limit=1&direction=backward", q, startNS, nowNS)
		body, err := grafanaGET(ctx, opts, path)
		if err != nil {
			return err
		}
		var payload map[string]any
		if err := json.Unmarshal(body, &payload); err != nil {
			return err
		}
		data, _ := payload["data"].(map[string]any)
		result, _ := data["result"].([]any)
		if len(result) > 0 {
			return nil
		}
	}
	return fmt.Errorf("no correlated loki logs found for trace_id %s", traceID)
}

func ioReadAll(r io.Reader) ([]byte, error) { return io.ReadAll(r) }
