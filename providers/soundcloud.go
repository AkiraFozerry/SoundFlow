package providers

import "context"

type SoundCloud struct{}

func (SoundCloud) Name() string { return "SoundCloud" }

func (SoundCloud) Search(ctx context.Context, query string, limit int) ([]Track, error) {
	return searchViaYtDlp(ctx, "scsearch", "SoundCloud", query, limit)
}
