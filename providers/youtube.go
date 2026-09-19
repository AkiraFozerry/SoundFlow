package providers

import "context"

type YouTube struct{}

func (YouTube) Name() string { return "YouTube" }

func (YouTube) Search(ctx context.Context, query string, limit int) ([]Track, error) {
	return searchViaYtDlp(ctx, "ytsearch", "YouTube", query, limit)
}
