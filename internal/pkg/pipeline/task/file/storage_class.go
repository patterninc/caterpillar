package file

import (
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

type storageClass = types.StorageClass

var (
	storageClasses      map[string]storageClass
	defaultStorageClass storageClass
)

func ensureStorageClasses() {
	vals := types.StorageClass("").Values()
	storageClasses = make(map[string]storageClass, len(vals))
	for _, v := range vals {
		storageClasses[string(v)] = v
	}
	defaultStorageClass = storageClasses["STANDARD"]
}

func validateStorageClass(c storageClass) error {
	if _, ok := storageClasses[string(c)]; ok {
		return nil
	}
	return fmt.Errorf("storage_class: unknown value %q", c)
}
