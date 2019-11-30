package velero

import (
	"context"
	"fmt"
	"time"

	veleroCR "github.com/openshift/managed-velero-operator/pkg/apis/managed/v1alpha1"
	"github.com/openshift/managed-velero-operator/pkg/gcs"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	configapiv1 "github.com/openshift/api/config/v1"

	gstorage "cloud.google.com/go/storage"
	"github.com/google/uuid"
)

const (
	bucketPrefix                    = "managed-velero-backups-"
)

var (
	UniformBucketLevelAccessEnabled = gstorage.UniformBucketLevelAccess{Enabled: true}
)

func (r *ReconcileVelero) provisionStorage(reqLogger logr.Logger, gcsClient *gstorage.Client, platformStatus *configapiv1.PlatformStatus, instance *veleroCR.Velero) (reconcile.Result, error) {
	var err error
	bucketLog := reqLogger.WithValues("Bucket.Name", instance.Status.S3Bucket.Name, "Project", platformStatus.GCP.ProjectID, "Region", platformStatus.GCP.Region)

	// We don't yet have a bucket name selected
	if instance.Status.S3Bucket.Name == "" {
		log.Info("No storage bucket defined")
		proposedName := generateBucketName(bucketPrefix)
		proposedBucketExists, err := gcs.DoesBucketExist(gcsClient, proposedName)
		if err != nil {
			return reconcile.Result{}, err
		}
		if proposedBucketExists {
			return reconcile.Result{}, fmt.Errorf("proposed bucket %s already exists, retrying", proposedName)
		}

		log.Info("Setting proposed bucket name", "Bucket.Name", proposedName)
		instance.Status.S3Bucket.Name = proposedName
		instance.Status.S3Bucket.Provisioned = false
		return reconcile.Result{}, r.statusUpdate(reqLogger, instance)
	}

	bucket := gcsClient.Bucket(instance.Status.S3Bucket.Name)
	var bucketAttrs *gstorage.BucketAttrs

	// We have a bucket name, but haven't kicked off provisioning of the bucket yet
	if !instance.Status.S3Bucket.Provisioned {
		bucketLog.Info("S3 bucket defined, but not provisioned")

		// Create storage bucket
		bucketLog.Info("Creating storage Bucket")
		bucketAttrs = &gstorage.BucketAttrs{
			Location: platformStatus.GCP.Region,
			UniformBucketLevelAccess: UniformBucketLevelAccessEnabled,
		}
		err := bucket.Create(context.TODO(), platformStatus.GCP.ProjectID, bucketAttrs)
		if err != nil {
			return reconcile.Result{}, fmt.Errorf("error occurred when creating bucket %v: %v", instance.Status.S3Bucket.Name, err)
		}
	}

	// Verify storage bucket exists
	bucketLog.Info("Verifing storage bucket exists")
	exists, err := gcs.DoesBucketExist(gcsClient, instance.Status.S3Bucket.Name)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("error occurred when verifying bucket %v: %v", instance.Status.S3Bucket.Name, err.Error())
	}
	if !exists {
		bucketLog.Error(nil, "storage bucket doesn't appear to exist")
		instance.Status.S3Bucket.Provisioned = false
		return reconcile.Result{}, r.statusUpdate(reqLogger, instance)
	}
	bucketAttrs, err = bucket.Attrs(context.TODO())
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("error occurred when retrieving bucket attributes %v: %v", instance.Status.S3Bucket.Name, err.Error())
	}

	// Enforce UniformBucketLevelAccess setting on bucket
	if !bucketAttrs.UniformBucketLevelAccess.Enabled {
		bucketLog.Info("Enforce UniformBucketLevelAccess setting on bucket")
		if _, err = bucket.Update(context.TODO(), gstorage.BucketAttrsToUpdate{UniformBucketLevelAccess: &UniformBucketLevelAccessEnabled}); err != nil {
			return reconcile.Result{}, fmt.Errorf("error occurred when enforcing UniformBucketLevelAccess to bucket %v: %v", instance.Status.S3Bucket.Name, err.Error())
		}
	}

	// Configure lifecycle rules on storage bucket
	// TODO(cblecker): make this work
	// bucketLog.Info("Enforcing storage bucket lifecycle rules on bucket")
	// err = s3.SetBucketLifecycle(s3Client, instance.Status.S3Bucket.Name)
	// if err != nil {
	// 	if aerr, ok := err.(awserr.Error); ok {
	// 		return reconcile.Result{}, fmt.Errorf("error occurred when configuring lifecycle rules on bucket %v: %v", instance.Status.S3Bucket.Name, aerr.Error())
	// 	}
	// 	return reconcile.Result{}, fmt.Errorf("error occurred when configuring lifecycle rules on bucket %v: %v", instance.Status.S3Bucket.Name, err.Error())
	// }

	// Make sure that labels are applied to buckets
	// TODO(cblecker): make this work
	// bucketLog.Info("Enforcing storage bucket labels on bucket")
	// err = s3.TagBucket(s3Client, instance.Status.S3Bucket.Name, defaultBackupStorageLocation)
	// if err != nil {
	// 	return reconcile.Result{}, fmt.Errorf("error occurred when tagging bucket %v: %v", instance.Status.S3Bucket.Name, err.Error())
	// }

	instance.Status.S3Bucket.Provisioned = true
	instance.Status.S3Bucket.LastSyncTimestamp = &metav1.Time{
		Time: time.Now(),
	}
	return reconcile.Result{}, r.statusUpdate(reqLogger, instance)
}

func generateBucketName(prefix string) string {
	id := uuid.New().String()
	return prefix + id
}
