package gcs

import (
	"context"

	gstorage "cloud.google.com/go/storage"
)

func DoesBucketExist(gcsClient *gstorage.Client, bucketName string) (bool, error) {
	_, err := gcsClient.Bucket(bucketName).Attrs(context.TODO())
	if err != nil {
		if err == gstorage.ErrBucketNotExist {
			return false, nil
		}

		return false, err
	}

	return true, nil
}
