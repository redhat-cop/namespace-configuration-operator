package common

import "github.com/scylladb/go-set/strset"

//DefaultExcludedPaths represents paths that are exlcuded by default in all resources
var DefaultExcludedPaths = []string{".metadata", ".status", ".spec.replicas"}

//DefaultExcludedPathsSet represents paths that are exlcuded by default in all resources
var DefaultExcludedPathsSet = strset.New(DefaultExcludedPaths...)
