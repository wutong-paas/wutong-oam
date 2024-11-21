// Copyright (C) 2014-2018 Wutong Co., Ltd.
// WUTONG, Application Management Platform

// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version. For any non-GPL usage of Wutong,
// one or multiple Commercial Licenses authorized by Wutong Co., Ltd.
// must be obtained first.

// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.

// You should have received a copy of the GNU General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

package docker

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/client"
	"github.com/sirupsen/logrus"
	"github.com/wutong-paas/wutong-oam/pkg/ram/v1alpha1"
	"github.com/wutong-paas/wutong-oam/pkg/util"
	"golang.org/x/net/context"
)

// ErrorNoAuth error no auth
var ErrorNoAuth = fmt.Errorf("pull image require docker login")

// ErrorNoImage error no image
var ErrorNoImage = fmt.Errorf("image not exist")

// ImagePull pull docker image
// timeout minutes of the unit
func ImagePull(dockerCli *client.Client, imageName string, username, password string, timeout int) (*types.ImageInspect, error) {
	var pullipo image.PullOptions
	if username != "" && password != "" {
		auth, err := EncodeAuthToBase64(registry.AuthConfig{Username: username, Password: password})
		if err != nil {
			logrus.Errorf("make auth base63 push image error: %s", err.Error())
			return nil, err
		}
		pullipo = image.PullOptions{
			RegistryAuth: auth,
		}
	} else {
		pullipo = image.PullOptions{}
	}
	rf, err := reference.ParseAnyReference(imageName)
	if err != nil {
		logrus.Errorf("reference image error: %s", err.Error())
		return nil, err
	}
	//最少一分钟
	if timeout < 1 {
		timeout = 1
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*time.Duration(timeout))
	defer cancel()
	//TODO: 使用1.12版本api的bug “repository name must be canonical”，使用rf.String()完整的镜像地址
	readcloser, err := dockerCli.ImagePull(ctx, rf.String(), pullipo)
	if err != nil {
		logrus.Debugf("image name: %s readcloser error: %v", imageName, err.Error())
		if strings.HasSuffix(err.Error(), "does not exist or no pull access") {
			return nil, fmt.Errorf("Image(%s) does not exist or no pull access", imageName)
		}
		return nil, err
	}
	defer readcloser.Close()
	dec := json.NewDecoder(readcloser)
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		var jm JSONMessage
		if err := dec.Decode(&jm); err != nil {
			if err == io.EOF {
				break
			}
			logrus.Debugf("error decoding jm(JSONMessage): %v", err)
			return nil, err
		}
		if jm.Error != nil {
			logrus.Debugf("error pulling image: %v", jm.Error)
			return nil, jm.Error
		}
	}
	ins, _, err := dockerCli.ImageInspectWithRaw(ctx, imageName)
	if err != nil {
		return nil, err
	}
	return &ins, nil
}

// ImageTag change docker image tag
func ImageTag(dockerCli *client.Client, source, target string, timeout int) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*time.Duration(timeout))
	defer cancel()
	if _, _, err := dockerCli.ImageInspectWithRaw(ctx, source); err != nil && client.IsErrNotFound(err) {
		// 本地没有该镜像，说明没有被 Load，则直接返回
		logrus.Errorf("imagetag imageService Get error: %s", err.Error())
		return util.ErrLocalImageNotFound
	}

	err := dockerCli.ImageTag(ctx, source, target)
	if err != nil {
		logrus.Debugf("image tag err: %s", err.Error())
		return err
	}
	return nil
}

// ImagePush push image to registry
// timeout minutes of the unit
func ImagePush(dockerCli *client.Client, imageName, username, password string, timeout int) error {
	if timeout < 1 {
		timeout = 1
	}
	_, err := reference.ParseNormalizedNamed(imageName)
	if err != nil {
		return err
	}
	var pushipo image.PushOptions
	if username != "" && password != "" {
		auth, err := EncodeAuthToBase64(registry.AuthConfig{Username: username, Password: password})
		if err != nil {
			logrus.Errorf("make auth base63 push image error: %s", err.Error())
			return err
		}
		pushipo = image.PushOptions{
			RegistryAuth: auth,
		}
	} else {
		pushipo = image.PushOptions{}
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*time.Duration(timeout))
	defer cancel()
	readcloser, err := dockerCli.ImagePush(ctx, imageName, pushipo)
	if err != nil {
		if strings.Contains(err.Error(), "does not exist") {
			return fmt.Errorf("Image(%s) does not exist", imageName)
		}
		return err
	}
	if readcloser != nil {
		defer readcloser.Close()
		dec := json.NewDecoder(readcloser)
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			var jm JSONMessage
			if err := dec.Decode(&jm); err != nil {
				if err == io.EOF {
					break
				}
				return err
			}
			if jm.Error != nil {
				return jm.Error
			}
		}
	}
	return nil
}

// TrustedImagePush push image to trusted registry
func TrustedImagePush(dockerCli *client.Client, imageName, user, pass string, timeout int) error {
	if err := CheckTrustedRepositories(imageName, user, pass); err != nil {
		return err
	}
	return ImagePush(dockerCli, imageName, user, pass, timeout)
}

// CheckTrustedRepositories check Repositories is exist ,if not create it.
func CheckTrustedRepositories(image, user, pass string) error {
	ref, err := reference.ParseNormalizedNamed(image)
	if err != nil {
		return err
	}
	var server string
	if reference.IsNameOnly(ref) {
		server = "docker.io"
	} else {
		server = reference.Domain(ref)
	}
	cli, err := createTrustedRegistryClient(server, user, pass)
	if err != nil {
		return err
	}
	var namespace, repName string
	infos := strings.Split(reference.TrimNamed(ref).String(), "/")
	if len(infos) == 3 && infos[0] == server {
		namespace = infos[1]
		repName = infos[2]
	}
	if len(infos) == 2 {
		namespace = infos[0]
		repName = infos[1]
	}
	_, err = cli.GetRepository(namespace, repName)
	if err != nil {
		if err.Error() == "resource does not exist" {
			rep := Repostory{
				Name:             repName,
				ShortDescription: image, // The maximum length is 140
				LongDescription:  fmt.Sprintf("push image for %s", image),
				Visibility:       "private",
			}
			if len(rep.ShortDescription) > 140 {
				rep.ShortDescription = rep.ShortDescription[0:140]
			}
			err := cli.CreateRepository(namespace, &rep)
			if err != nil {
				return fmt.Errorf("create repostory error,%s", err.Error())
			}
			return nil
		}
		return fmt.Errorf("get repostory error,%s", err.Error())
	}
	return err
}

// EncodeAuthToBase64 serializes the auth configuration as JSON base64 payload
func EncodeAuthToBase64(authConfig registry.AuthConfig) (string, error) {
	buf, err := json.Marshal(authConfig)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(buf), nil
}

// ImageInspectWithRaw get image inspect
func ImageInspectWithRaw(dockerCli *client.Client, image string) (*types.ImageInspect, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ins, _, err := dockerCli.ImageInspectWithRaw(ctx, image)
	if err != nil {
		return nil, err
	}
	return &ins, nil
}

// ImageSave save image to tar file
// destination destination file name eg. /tmp/xxx.tar
func ImageSave(dockerCli *client.Client, image, destination string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	rc, err := dockerCli.ImageSave(ctx, []string{image})
	if err != nil {
		return err
	}
	defer rc.Close()
	return CopyToFile(destination, rc)
}

// MultiImageSave save multi image to tar file
// destination destination file name eg. /tmp/xxx.tar
func MultiImageSave(ctx context.Context, dockerCli *client.Client, destination string, images ...string) error {
	rc, err := dockerCli.ImageSave(ctx, images)
	if err != nil {
		return err
	}
	defer rc.Close()
	return CopyToFile(destination, rc)
}

// ImageLoad load image from  tar file
// destination destination file name eg. /tmp/xxx.tar
func ImageLoad(dockerCli *client.Client, tarFile string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	reader, err := os.OpenFile(tarFile, os.O_RDONLY, 0644)
	if err != nil {
		return err
	}
	defer reader.Close()

	rc, err := dockerCli.ImageLoad(ctx, reader, false)
	if err != nil {
		return err
	}
	if rc.Body != nil {
		defer rc.Body.Close()
		dec := json.NewDecoder(rc.Body)
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			var jm JSONMessage
			if err := dec.Decode(&jm); err != nil {
				if err == io.EOF {
					break
				}
				return err
			}
			if jm.Error != nil {
				return jm.Error
			}
		}
	}

	return nil
}

// ImageImport save image to tar file
// source source file name eg. /tmp/xxx.tar
func ImageImport(dockerCli *client.Client, imageName, source string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	file, err := os.Open(source)
	if err != nil {
		return err
	}
	defer file.Close()

	isource := image.ImportSource{
		Source:     file,
		SourceName: "-",
	}

	options := image.ImportOptions{}

	readcloser, err := dockerCli.ImageImport(ctx, isource, imageName, options)
	if err != nil {
		return err
	}
	if readcloser != nil {
		defer readcloser.Close()
		dec := json.NewDecoder(readcloser)
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			var jm JSONMessage
			if err := dec.Decode(&jm); err != nil {
				if err == io.EOF {
					break
				}
				return err
			}
			if jm.Error != nil {
				return jm.Error
			}
		}
	}
	return nil
}

// CopyToFile writes the content of the reader to the specified file
func CopyToFile(outfile string, r io.Reader) error {
	// We use sequential file access here to avoid depleting the standby list
	// on Windows. On Linux, this is a call directly to os.CreateTemp
	tmpFile, err := os.OpenFile(path.Join(filepath.Dir(outfile), ".docker_temp_"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer tmpFile.Close()
	tmpPath := tmpFile.Name()
	_, err = io.Copy(tmpFile, r)
	if err != nil {
		os.Remove(tmpPath)
		return err
	}
	if err = os.Rename(tmpPath, outfile); err != nil {
		os.Remove(tmpPath)
		return err
	}
	return nil
}

// ImageRemove remove image
func ImageRemove(dockerCli *client.Client, imageName string) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	_, err := dockerCli.ImageRemove(ctx, imageName, image.RemoveOptions{Force: true})
	return err
}

// GetTagFromNamedRef get image tag by name
func GetTagFromNamedRef(ref reference.Named) string {
	if digested, ok := ref.(reference.Digested); ok {
		return digested.Digest().String()
	}
	ref = reference.TagNameOnly(ref)
	if tagged, ok := ref.(reference.Tagged); ok {
		return tagged.Tag()
	}
	return ""
}

// NewImageName new image name
func NewImageName(source string, hubInfo v1alpha1.ImageInfo) (string, error) {
	var nameTag string
	if strings.Contains(source, "/") {
		nameTag = source[strings.LastIndex(source, "/")+1:]
	}
	newImageName := fmt.Sprintf("%s/%s", hubInfo.HubURL, nameTag)
	if hubInfo.Namespace != "" {
		newImageName = fmt.Sprintf("%s/%s/%s", hubInfo.HubURL, hubInfo.Namespace, nameTag)
	}
	return newImageName, nil
}

// GetOldSaveImageName get old save image name before V5.3
func GetOldSaveImageName(source string, withDomain bool) (string, error) {
	ref, err := reference.ParseAnyReference(source)
	if err != nil {
		return "", err
	}
	name, err := reference.ParseNamed(ref.String())
	if err != nil {
		return "", err
	}
	var nameTag string
	if strings.Contains(source, "/") {
		nameTag = source[strings.LastIndex(source, "/")+1:]
	}
	if withDomain {
		return fmt.Sprintf("%s/%s", reference.Domain(name), nameTag), nil
	}
	return nameTag, nil
}
