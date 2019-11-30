# Project specific values
OPERATOR_NAME?=managed-velero-operator
OPERATOR_NAMESPACE?=openshift-velero

IMAGE_REGISTRY?=quay.io
IMAGE_REPOSITORY?=cblecker
IMAGE_NAME?=$(OPERATOR_NAME)-gcp

VERSION_MAJOR?=0
VERSION_MINOR?=1
