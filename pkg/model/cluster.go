package model

import (
	"github.com/KubeOperator/KubeOperator/pkg/constant"
	"github.com/KubeOperator/KubeOperator/pkg/db"
	"github.com/KubeOperator/KubeOperator/pkg/model/common"
	"github.com/KubeOperator/kobe/api"
	uuid "github.com/satori/go.uuid"
)

type Cluster struct {
	common.BaseModel
	ID       string        `json:"_"`
	Name     string        `json:"name"`
	SpecID   string        `json:"_"`
	SecretID string        `json:"_"`
	StatusID string        `json:"_"`
	Spec     ClusterSpec   `gorm:"save_associations:false" json:"spec"`
	Secret   ClusterSecret `gorm:"save_associations:false" json:"_"`
	Status   ClusterStatus `gorm:"save_associations:false" json:"_"`
	Nodes    []ClusterNode `gorm:"save_associations:false" json:"_"`
}

func (c Cluster) TableName() string {
	return "ko_cluster"
}

func (c *Cluster) BeforeCreate() error {
	c.ID = uuid.NewV4().String()
	tx := db.DB.Begin()
	if err := tx.Create(&c.Spec).Error; err != nil {
		tx.Rollback()
		return err
	}
	if err := tx.Create(&c.Status).Error; err != nil {
		tx.Rollback()
		return err
	}
	if err := tx.Create(&c.Secret).Error; err != nil {
		tx.Rollback()
		return err
	}
	c.SpecID = c.Spec.ID
	c.StatusID = c.Status.ID
	c.SecretID = c.Secret.ID
	for i, _ := range c.Nodes {
		c.Nodes[i].ClusterID = c.ID
		if err := tx.Create(&c.Nodes[i]).Error; err != nil {
			tx.Rollback()
			return err
		}
	}
	tx.Commit()
	return nil
}

func (c Cluster) BeforeDelete() error {
	var cluster Cluster
	if err := db.DB.
		First(&Cluster{ID: c.ID}).
		Preload("Status").
		Preload("Spec").
		Preload("Nodes").
		Find(&cluster).Error; err != nil {
		return err
	}
	tx := db.DB.Begin()
	if err := tx.Where(ClusterSpec{ID: cluster.SpecID}).
		Delete(ClusterSpec{}).Error; err != nil {
		tx.Rollback()
		return err
	}
	if err := tx.Where(ClusterStatus{ID: cluster.StatusID}).
		Delete(ClusterStatus{}).Error; err != nil {
		tx.Rollback()
		return err
	}
	if err := tx.Where(ClusterSecret{ID: cluster.SecretID}).
		Delete(ClusterSecret{}).Error; err != nil {
		tx.Rollback()
		return err
	}
	for _, node := range cluster.Nodes {
		if err := tx.Where(ClusterNode{ID: node.ID}).
			Delete(ClusterNode{}).Error; err != nil {
			tx.Rollback()
			return err
		}
	}
	tx.Commit()
	return nil
}

func (c Cluster) ParseInventory() api.Inventory {
	var masters []string
	var workers []string
	var chrony []string
	var hosts []*api.Host
	for _, node := range c.Nodes {
		hosts = append(hosts, node.ToKobeHost())
		switch node.Role {
		case constant.NodeRoleNameMaster:
			masters = append(masters, node.Name)
		case constant.NodeRoleNameWorker:
			workers = append(workers, node.Name)
		}
	}
	if len(masters) > 0 {
		chrony = append(chrony, masters[0])
	}
	return api.Inventory{
		Hosts: hosts,
		Groups: []*api.Group{
			{
				Name:     "kubernetes-master",
				Hosts:    masters,
				Children: []string{},
				Vars:     map[string]string{},
			},
			{
				Name:     "kubernetes-worker",
				Hosts:    workers,
				Children: []string{},
				Vars:     map[string]string{},
			},
			{
				Name:     "new-worker",
				Hosts:    []string{},
				Children: []string{},
				Vars:     map[string]string{},
			}, {

				Name:     "lb",
				Hosts:    []string{},
				Children: []string{},
				Vars:     map[string]string{},
			},
			{
				Name:     "etcd",
				Hosts:    masters,
				Children: []string{"master"},
				Vars:     map[string]string{},
			}, {
				Name:     "chrony",
				Hosts:    chrony,
				Children: []string{},
				Vars:     map[string]string{},
			},
		},
	}
}