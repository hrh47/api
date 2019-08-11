package storage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gocloud.dev/blob"
	_ "gocloud.dev/blob/fileblob"
	_ "gocloud.dev/blob/gcsblob"

	"github.com/hiconvo/api/utils/secrets"
)

var avatarBucketName string
var urlPrefix string

func init() {
	// For local development
	localpath, err := filepath.Abs("./.local-object-store/")
	if err != nil {
		panic(err)
	}
	fallBackPath := fmt.Sprintf("file://%s", localpath)

	avatarBucketName = secrets.Get("AVATAR_BUCKET_NAME", fallBackPath)

	// Make sure the storage dir exists when doing local dev
	if avatarBucketName[:8] == "file:///" {
		if err := os.MkdirAll(localpath, 0777); err != nil {
			panic(err)
		}

		urlPrefix = fallBackPath + "/"
	} else {
		urlPrefix = "https://storage.googleapis.com/convo-avatars/"
	}
}

func GetAvatarBucket(ctx context.Context) (*blob.Bucket, error) {
	return blob.OpenBucket(ctx, avatarBucketName)
}

func GetFullAvatarURL(object string) string {
	return urlPrefix + object
}

func GetKeyFromAvatarURL(url string) string {
	if url == "" {
		return "null-key"
	}

	ss := strings.Split(url, "/")
	return ss[len(ss)-1]
}
