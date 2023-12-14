package dataprovider

import (
	"time"

	"github.com/forscht/ddrv/pkg/ddrv"
)

var provider DataProvider

type DataProvider interface {
	Get(id, parent string) (*File, error)
	GetChild(id string) ([]*File, error)
	Create(name, parent string, isDir bool) (*File, error)
	Update(id, parent string, file *File) (*File, error)
	Delete(id, parent string) error
	GetNodes(id string) ([]ddrv.Node, error)
	CreateNodes(id string, nodes []ddrv.Node) error
	DeleteNodes(id string) error
	Stat(path string) (*File, error)
	Ls(path string, limit int, offset int) ([]*File, error)
	Touch(path string) error
	Mkdir(path string) error
	Rm(path string) error
	Mv(name, newname string) error
	CHTime(path string, time time.Time) error
	Close() error
}

func Load(dp DataProvider) {
	provider = dp
}

func Get(id, parent string) (*File, error) {
	return provider.Get(id, parent)
}

func GetChild(id string) ([]*File, error) {
	return provider.GetChild(id)
}

func Create(name, parent string, isDir bool) (*File, error) {
	return provider.Create(name, parent, isDir)
}

func Update(id, parent string, file *File) (*File, error) {
	return provider.Update(id, parent, file)
}

func Delete(id, parent string) error {
	return provider.Delete(id, parent)
}

func GetNodes(id string) ([]ddrv.Node, error) {
	return provider.GetNodes(id)
}

func CreateNodes(id string, nodes []ddrv.Node) error {
	return provider.CreateNodes(id, nodes)
}

func DeleteNodes(id string) error {
	return provider.DeleteNodes(id)
}

func Stat(path string) (*File, error) {
	return provider.Stat(path)
}

func Ls(path string, limit int, offset int) ([]*File, error) {
	return provider.Ls(path, limit, offset)
}

func Touch(path string) error {
	return provider.Touch(path)
}

func Mkdir(path string) error {
	return provider.Mkdir(path)
}

func Rm(path string) error {
	return provider.Rm(path)
}

func Mv(name, newname string) error {
	return provider.Mv(name, newname)
}

func ChMTime(path string, time time.Time) error {
	return provider.CHTime(path, time)
}
