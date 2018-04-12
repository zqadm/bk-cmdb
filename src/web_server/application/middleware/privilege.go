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

package middleware

import (
	"configcenter/src/common"
	"configcenter/src/common/blog"
	"configcenter/src/common/http/httpclient"
	"configcenter/src/common/paraparse"
	webCommon "configcenter/src/web_server/common"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
)

type Privilege struct {
	UserName string
	APIAddr  string
	OwnerID  string
	language string
	httpCli  *httpclient.HttpClient
}

func NewPrivilege(userName, APIAddr, ownerID, language string) (*Privilege, error) {
	privi := new(Privilege)
	privi.UserName = userName
	privi.APIAddr = APIAddr
	privi.OwnerID = ownerID
	privi.httpCli = httpclient.NewHttpClient()
	privi.httpCli.SetHeader(common.BKHTTPHeaderUser, userName)
	privi.httpCli.SetHeader(common.BKHTTPLanguage, language)
	privi.httpCli.SetHeader(common.BKHTTPOwnerID, ownerID)
	return privi, nil
}

type RolePriResult struct {
	Result  bool        `json:"result"`
	Code    int         `json:"bk_error_code"`
	Message interface{} `json:"bk_error_msg"`
	Data    []string    `json:"data"`
}

type RoleAppResult struct {
	Result  bool                     `json:"result"`
	Code    int                      `json:"bk_error_code"`
	Message interface{}              `json:"bk_error_msg"`
	Data    []map[string]interface{} `json:"data"`
}

type SearchAppResult struct {
	Result  bool        `json:"result"`
	Code    int         `json:"bk_error_code"`
	Message interface{} `json:"bk_error_msg"`
	Data    AppResult   `json:"data"`
}

type AppResult struct {
	Count int                      `json:"count"`
	Info  []map[string]interface{} `json:"info"`
}

func ValidPrivilege() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(200, gin.H{
			"bk_error_msg": "pong",
		})
	}
}

// GetRolePrivilege get role privilege
func (p *Privilege) GetRolePrivilege(objID string, role string) []string {
	url := fmt.Sprintf("%s/api/%s/topo/privilege/%s/%s/%s", p.APIAddr, webCommon.API_VERSION, p.OwnerID, objID, role)
	blog.Info("get role pri url: %s", url)
	getResult, err := p.httpCli.GET(url, nil, nil)
	if nil != err {
		blog.Error("get role privilege return error: %v", err)
		return nil
	}
	blog.Info("get role privilege return: %s", string(getResult))
	var resultData RolePriResult
	err = json.Unmarshal([]byte(getResult), &resultData)
	if nil != err || false == resultData.Result {
		blog.Error("get role privilege json error: %v", err)
		return nil
	}
	return resultData.Data

}

//GetAppRole get app role
func (p *Privilege) GetAppRole() []string {
	result := make([]string, 0)
	url := fmt.Sprintf("%s/api/%s/object/attr/search", p.APIAddr, webCommon.API_VERSION)
	cond := make(map[string]interface{})
	cond[common.BKPropertyTypeField] = "objuser"
	cond[common.BKObjIDField] = common.BKInnerObjIDApp
	data, _ := json.Marshal(cond)
	blog.Info("get app role  url: %s", url)
	blog.Info("get app role  content: %s", data)
	getResult, err := p.httpCli.POST(url, nil, data)
	if nil != err {
		blog.Error("get app role return error: %v", err)
		return nil
	}
	blog.Info("get app role return: %s", string(getResult))
	var resultData RoleAppResult
	err = json.Unmarshal([]byte(getResult), &resultData)
	if nil != err || false == resultData.Result {
		blog.Error("get role privilege json error: %v", err)
		return nil
	}
	for _, i := range resultData.Data {
		propertyID, ok := i[common.BKPropertyIDField].(string)
		if false == ok {
			continue
		}
		result = append(result, propertyID)
	}
	return result
}

// GetUserPrivilegeApp get user privilege app
func (p *Privilege) GetUserPrivilegeApp(appRole []string) map[int64][]string {
	url := fmt.Sprintf("%s/api/%s/biz/search/%s", p.APIAddr, webCommon.API_VERSION, p.OwnerID)
	orCond := make([]interface{}, 0)
	allCond := make(map[string]interface{})
	condition := make(map[string]interface{})
	for _, role := range appRole {
		cell := make(map[string]interface{})
		d := make(map[string]interface{})
		cell[common.BKDBLIKE] = p.UserName
		d[role] = cell
		orCond = append(orCond, d)
	}
	allCond[common.BKDBOR] = orCond
	condition["condition"] = allCond
	condition["native"] = 1
	data, _ := json.Marshal(condition)
	blog.Info("search app role  url: %s", url)
	blog.Info("search app role  content: %s", data)
	getResult, err := p.httpCli.POST(url, nil, data)
	blog.Info("search app role  return: %s", string(getResult))
	if nil != err {
		blog.Error("search app role return error: %v", err)
		return nil
	}
	var resultData SearchAppResult
	err = json.Unmarshal([]byte(getResult), &resultData)
	if nil != err || false == resultData.Result || 0 == resultData.Data.Count {
		blog.Error("search role privilege json error: %v", err)
		blog.Error("search role privilege result error: %v", resultData.Result)
		blog.Error("search role privilege data error: %v", resultData.Data.Count)
		return nil
	}
	userRole := make(map[int64][]string)
	for _, i := range resultData.Data.Info {
		appID64 := i[common.BKAppIDField].(float64)
		appID := int64(appID64)
		userRoleArr := make([]string, 0)
		for _, j := range appRole {
			roleData, ok := i[j]
			if false == ok {
				continue
			}
			roleStr := roleData.(string)
			if false == ok {
				continue
			}
			roleArr := strings.Split(roleStr, ",")
			for _, k := range roleArr {
				if k == p.UserName {
					userRoleArr = append(userRoleArr, j)
				}
			}
			userRole[appID] = userRoleArr
		}
	}
	return userRole
}

//GetUserPrivilegeConfig get user privilege config
func (p *Privilege) GetUserPrivilegeConfig() (map[string][]string, []string) {
	url := fmt.Sprintf("%s/api/%s/topo/privilege/user/detail/%s/%s", p.APIAddr, webCommon.API_VERSION, p.OwnerID, p.UserName)
	blog.Info("get user privilege config  url: %s", url)
	getResult, err := p.httpCli.GET(url, nil, nil)
	blog.Info("get user privilege config  return: %s", getResult)
	if nil != err {
		blog.Error("get user privilege return error: %v", err)
		return nil, nil
	}
	blog.Info("get app role return: %s", string(getResult))
	var resultData params.UserPriviResult
	err = json.Unmarshal([]byte(getResult), &resultData)
	if nil != err || false == resultData.Result {
		blog.Error("get user privilege json error: %v", err)
		return nil, nil
	}
	sysConfig := make([]string, 0)
	modelConfig := make(map[string][]string, 0)
	for _, i := range resultData.Data.SysConfig.BackConfig {
		sysConfig = append(sysConfig, i)
	}

	for _, i := range resultData.Data.SysConfig.Globalbusi {
		sysConfig = append(sysConfig, i)
	}
	for _, j := range resultData.Data.ModelConfig {
		for m, n := range j {
			modelConfig[m] = n
		}
	}
	return modelConfig, sysConfig
}
