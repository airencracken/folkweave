package folkweave

import (
	"context"
	"database/sql"
	"errors"
	"net/url"
	"os"
)

// NewReadOnlyDemo always creates a separate, disposable database. The caller
// closes it with the returned cleanup function after stopping the server.
func NewReadOnlyDemo(ctx context.Context, config Config) (*App, func() error, error) {
	dir, err := os.MkdirTemp("", "folkweave-sample-")
	if err != nil {
		return nil, nil, err
	}
	store, err := openSample(ctx, dir)
	if err != nil {
		return nil, nil, errors.Join(err, os.RemoveAll(dir))
	}
	cleanup := func() error { return errors.Join(store.Close(), os.RemoveAll(dir)) }
	app, err := newApp(store, config, true)
	if err != nil {
		return nil, nil, errors.Join(err, cleanup())
	}
	return app, cleanup, nil
}

func openSample(ctx context.Context, dir string) (*Store, error) {
	store, err := Open(dir)
	if err != nil {
		return nil, err
	}
	if err := errors.Join(prepareSample(ctx, store), store.Close()); err != nil {
		return nil, err
	}
	dsn := (&url.URL{Scheme: "file", Path: store.Path, RawQuery: "mode=ro"}).String()
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if err := db.PingContext(ctx); err != nil {
		return nil, errors.Join(err, db.Close())
	}
	return &Store{db: db, Path: store.Path}, nil
}

func prepareSample(ctx context.Context, store *Store) error {
	if err := SeedDemo(ctx, store); err != nil {
		return err
	}
	// Generate derivatives before accepting requests. Sample visitors only
	// read existing fixture data, even when opening a photo preview.
	timeline, err := store.Posts(ctx, 0, "", false, 0)
	if err != nil {
		return err
	}
	for _, post := range timeline.Posts {
		for _, photo := range post.Photos {
			for _, width := range []int{320, 640, 960} {
				if _, err := store.PhotoPreview(ctx, photo.ID, width); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
