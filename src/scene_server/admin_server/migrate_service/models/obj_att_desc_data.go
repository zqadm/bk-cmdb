/*
 * Tencent is pleased to support the open source community by making 蓝鲸 available.
 * Copyright (C) 2017-2018 THL A29 Limited, a Tencent company. All rights reserved.
 * Licensed under the MIT License (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the License at
 * http://opensource.org/licenses/MIT
 * Unless required by applicable law or agreed to in writing, software distributed under
 * the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
 * either express or implied. See the License for the specific language governing permissions and
 * limitations under the License.
 */

package models

import (
	"configcenter/src/common"
	"configcenter/src/common/blog"
	"configcenter/src/scene_server/admin_server/migrate_service/data"
	"configcenter/src/source_controller/api/metadata"
	dbStorage "configcenter/src/storage"
	"time"
)

func AddObjAttDescData(tableName, ownerID string, metaCli dbStorage.DI) error {
	blog.Errorf("add data for  %s table ", tableName)
	rows := getObjAttDescData(ownerID)
	for _, row := range rows {
		selector := map[string]interface{}{
			common.BKObjIDField:      row.ObjectID,
			common.BKPropertyIDField: row.PropertyID,
			common.BKOwnerIDField:    row.OwnerID,
		}
		isExist, err := metaCli.GetCntByCondition(tableName, selector)
		if nil != err {
			blog.Errorf("add data for  %s table error  %s", tableName, err)
			return err
		}
		if isExist > 0 {
			continue
		}
		id, err := metaCli.GetIncID(tableName)
		if nil != err {
			blog.Errorf("add data for  %s table error  %s", tableName, err)
			return err
		}
		row.ID = int(id)
		_, err = metaCli.Insert(tableName, row)
		if nil != err {
			blog.Errorf("add data for  %s table error  %s", tableName, err)
			return err
		}
	}

	blog.Errorf("add data for  %s table  ", tableName)
	return nil
}

func getObjAttDescData(ownerID string) []*metadata.ObjectAttDes {

	dataRows := data.AppRow()
	dataRows = append(dataRows, data.SetRow()...)
	dataRows = append(dataRows, data.ModuleRow()...)
	dataRows = append(dataRows, data.HostRow()...)
	dataRows = append(dataRows, data.ProcRow()...)
	dataRows = append(dataRows, data.PlatRow()...)
	t := new(time.Time)
	*t = time.Now()
	for _, r := range dataRows {
		r.OwnerID = ownerID
		r.IsPre = true
		if false != r.Editable {
			r.Editable = true
		}
		r.IsReadOnly = false
		r.CreateTime = t
		r.Creator = common.CCSystemOperatorUserName
		r.LastTime = r.CreateTime
		r.Description = ""

	}

	return dataRows

}

func AlterObjAttrDesTable(tableName string, metaCli dbStorage.DI) error {
	addCols := []*dbStorage.Column{
		dbStorage.GetMongoColumn("unit", ""),        //{Name: "Unit", Ext: " varchar(32) NOT NULL COMMENT '单位'"},
		dbStorage.GetMongoColumn("placeholder", ""), //.Column{Name: "Placeholder", Ext: " varchar(512) NOT NULL COMMENT '提示信息'"},
	}
	for _, c := range addCols {
		bl, err := metaCli.HasFields(tableName, c.Name)

		if nil != err {
			blog.Errorf("check  column %s is exist for  %s table error  %s", c.Name, tableName, err)
			return err
		}
		if !bl {
			err = metaCli.AddColumn(tableName, c)
			if nil != err {
				blog.Errorf("add column for  %s table error  %s", tableName, err)
				return err
			}
		}

	}
	return nil
}
