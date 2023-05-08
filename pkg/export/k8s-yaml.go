package export

import (
	"fmt"
	"os"
	"path"

	"github.com/sirupsen/logrus"
	"github.com/wutong-paas/wutong-oam/pkg/ram/v1alpha1"
	"github.com/wutong-paas/wutong-oam/pkg/util/image"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

type k8sYamlExporter struct {
	logger      *logrus.Logger
	ram         v1alpha1.WutongApplicationConfig
	imageClient image.Client
	mode        string
	homePath    string
	exportPath  string
}

func (y *k8sYamlExporter) Export() (*Result, error) {
	y.logger.Infof("start export app %s to k8s yaml spec", y.ram.AppName)
	if err := SaveComponents(y.ram, y.imageClient, y.exportPath, y.logger, []string{}); err != nil {
		y.logger.Errorf("k8s yaml export save component failure %v", err)
		return nil, err
	}
	y.logger.Infof("success save components")
	// Save plugin attachments
	if err := SavePlugins(y.ram, y.imageClient, y.exportPath, y.logger); err != nil {
		return nil, err
	}
	y.logger.Infof("success save plugins")
	if err := y.init(); err != nil {
		return nil, err
	}
	packageName := fmt.Sprintf("%s-%s-yaml.tar.gz", y.ram.AppName, y.ram.AppVersion)
	name, err := Packaging(packageName, y.homePath, y.exportPath)
	if err != nil {
		err = fmt.Errorf("failed to package app %s: %s", packageName, err.Error())
		y.logger.Error(err)
		return nil, err
	}
	y.logger.Infof("success export app " + y.ram.AppName)
	return &Result{PackagePath: path.Join(y.homePath, name), PackageName: name}, nil
}

func (y *k8sYamlExporter) init() error {
	yamlPath := path.Join(y.exportPath, y.ram.AppName)
	return y.writeYaml(yamlPath)
}

func (y *k8sYamlExporter) writeYaml(yamlPath string) error {
	for _, k8sResource := range y.ram.K8sResources {
		var unstructuredObject unstructured.Unstructured
		err := yaml.Unmarshal([]byte(k8sResource.Content), &unstructuredObject)
		if err != nil {
			return err
		}
		unstructuredObject.SetNamespace("")
		unstructuredObject.SetResourceVersion("")
		unstructuredObject.SetCreationTimestamp(metav1.Time{})
		unstructuredObject.SetUID("")
		unstructuredYaml, err := yaml.Marshal(unstructuredObject)
		if err != nil {
			return err
		}
		err = y.write(path.Join(yamlPath, fmt.Sprintf("%v.yaml", unstructuredObject.GetKind())), unstructuredYaml)
		if err != nil {
			return err
		}
	}
	return nil
}

func (y *k8sYamlExporter) write(yamlPath string, meta []byte) error {
	var fl *os.File
	var err error
	if CheckFileExist(yamlPath) {
		fl, err = os.OpenFile(yamlPath, os.O_APPEND|os.O_WRONLY, 0755)
		if err != nil {
			return err
		}
	} else {
		fl, err = os.Create(yamlPath)
		if err != nil {
			return err
		}
	}
	defer fl.Close()
	n, err := fl.Write(append(meta, []byte("\n---\n")...))
	if err != nil {
		return err
	}
	if n < len(append(meta, []byte("\n---")...)) {
		return fmt.Errorf("write insufficient length")
	}
	return nil
}
