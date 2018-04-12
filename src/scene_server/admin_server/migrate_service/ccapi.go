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

package ccapi

import (
	"configcenter/src/common/blog"
	"configcenter/src/common/core/cc/api"
	"configcenter/src/common/core/cc/config"
	"configcenter/src/common/errors"
	"configcenter/src/common/http/httpserver"
	"configcenter/src/common/rdapi"
	"configcenter/src/common/types"
	"sync"
	"time"
	// migrateCommon "configcenter/src/scene_server/admin_server/common"
	confCenter "configcenter/src/scene_server/admin_server/migrate_service/config"
	"configcenter/src/scene_server/admin_server/migrate_service/rdiscover"
	//"time"
)

//CCAPIServer define data struct of bcs ccapi server
type CCAPIServer struct {
	conf     *config.CCAPIConfig
	httpServ *httpserver.HttpServer
	rd       *rdiscover.RegDiscover
	cfCenter *confCenter.ConfCenter
}

func NewCCAPIServer(conf *config.CCAPIConfig) (*CCAPIServer, error) {
	s := &CCAPIServer{}

	//config
	s.conf = conf
	addr, _ := s.conf.GetAddress()
	port, _ := s.conf.GetPort()

	//http server
	s.httpServ = httpserver.NewHttpServer(port, addr, "")

	a := api.NewAPIResource()
	a.SetConfig(s.conf)
	a.InitAction()

	configctx, _ := a.ParseConfig()
	regDiscAddrs := configctx["register-server.addrs"]
	confCenterAddrs := configctx["config-server.addrs"]

	//RDiscover
	s.rd = rdiscover.NewRegDiscover(regDiscAddrs, addr, port, false)
	a.AddrSrv = s.rd

	//ConfCenter
	s.cfCenter = confCenter.NewConfCenter(confCenterAddrs)

	return s, nil
}

//Stop the ccapi server
func (ccAPI *CCAPIServer) Stop() error {
	return nil
}

//Start the ccapi server
func (ccAPI *CCAPIServer) Start() error {
	chErr := make(chan error, 3)

	a := api.NewAPIResource()

	config, _ := a.ParseConfig()
	confDir := config["confs.dir"]
	errres := config["errors.res"]

	// configure center
	err := ccAPI.cfCenter.Start(confDir, errres)
	if err != nil {
		blog.Errorf("configure center module start failed!. err:%s", err.Error())
		return err
	}

	//http server
	ccAPI.initHttpServ()

	// load the errors resource
	if errorres, ok := config["errors.res"]; ok {
		if errif, err := errors.New(errorres); nil != err {
			blog.Error("failed to create errors object, error info is  %s ", err.Error())
			chErr <- err
		} else {
			a.Error = errif
		}
	} else {
		for {
			errcode := ccAPI.cfCenter.GetLanguageCxt()
			if errcode == nil {
				blog.Warnf("fail to get language package, will get again")
				time.Sleep(time.Second * 2)
				continue
			} else {
				errif := errors.NewFromCtx(errcode)
				a.Error = errif
				break
			}
		}
	}

	a.TopoAPI = rdapi.GetRdAddrSrvHandle(types.CC_MODULE_TOPO, a.AddrSrv)
	a.ProcAPI = rdapi.GetRdAddrSrvHandle(types.CC_MODULE_PROC, a.AddrSrv)

	err = a.GetDataCli(config, "mongodb")
	if err != nil {
		blog.Error("connect mongodb error exit! err:%s", err.Error())
		return err
	}

	go func() {
		err := ccAPI.rd.Start()
		blog.Errorf("rdiscover start failed! err:%s", err.Error())
		chErr <- err
	}()

	waitfunc := func(f func() string, wg *sync.WaitGroup) {
		for {
			if f() != "" {
				wg.Done()
				return
			}
			time.Sleep(time.Millisecond * 500)
		}
	}
	all := []string{
		types.CC_MODULE_PROC,
		types.CC_MODULE_TOPO,
	}

	wg := &sync.WaitGroup{}
	for _, module := range all {
		wg.Add(1)
		go waitfunc(rdapi.GetRdAddrSrvHandle(module, a.AddrSrv), wg)
	}

	go func() {
		wg.Wait()
		err := ccAPI.httpServ.ListenAndServe()
		blog.Error("http listen and serve failed! err:%s", err.Error())
		chErr <- err
	}()
	select {
	case err := <-chErr:
		blog.Error("exit! err:%s", err.Error())
		return err
	}

	return nil
}

func (ccAPI *CCAPIServer) initHttpServ() error {
	a := api.NewAPIResource()

	ccAPI.httpServ.RegisterWebServer("/migrate/{version}", rdapi.GlobalFilter(types.CC_MODULE_PROC, types.CC_MODULE_TOPO), a.Actions)

	return nil
}
