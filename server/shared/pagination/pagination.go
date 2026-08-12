package pagination

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"time"
)

var ErrInvalidCursor = errors.New("pagination: invalid cursor")

type VideoCursor struct {
	CreatedAt time.Time `json:"created_at"`
	ID        string    `json:"id"`
	Ver       int       `json:"ver"`
}

type ObjectCursor struct {
	UploadedAt time.Time `json:"uploaded_at"`
	ID         string    `json:"id"`
	Ver        int       `json:"ver"`
}

func EncodeCursor(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

func DecodeCursor[T any](s string) (T, error) {
	var v T
	if s == "" {
		return v, nil
	}
	decoded, err := base64.URLEncoding.DecodeString(s)
	if err != nil {
		return v, ErrInvalidCursor
	}
	if err := json.Unmarshal(decoded, &v); err != nil {
		return v, ErrInvalidCursor
	}
	return v, nil
}
