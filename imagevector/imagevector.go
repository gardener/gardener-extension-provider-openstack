// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package imagevector

import (
	_ "embed"

	"github.com/gardener/gardener/pkg/utils/imagevector"
	"k8s.io/apimachinery/pkg/util/runtime"
)

const (
	// TerraformerImageName is the name of the Terraformer image.
	TerraformerImageName = "terraformer"
)

var (
	// ImagesYAML contains the content of the images.yaml file
	//go:embed images.yaml
	imagesYAML string
	// ImageVector is the image vector that contains all the needed images.
	imageVector imagevector.ImageVector
)

func init() {
	var (
		err      error
		caBundle *imagevector.CABundle
	)

	imageVector, caBundle, err = imagevector.Read([]byte(imagesYAML))
	runtime.Must(err)

	imageVector, _, err = imagevector.WithEnvOverride(imageVector, caBundle, imagevector.OverrideEnv)
	runtime.Must(err)
}

// ImageVector is the image vector that contains all the needed images.
func ImageVector() imagevector.ImageVector {
	return imageVector
}
