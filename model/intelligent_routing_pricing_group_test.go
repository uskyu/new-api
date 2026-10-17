package model

import (
  "testing"
  "github.com/QuantumNous/new-api/common"
  "github.com/QuantumNous/new-api/dto"
  "github.com/QuantumNous/new-api/setting"
  "github.com/QuantumNous/new-api/setting/ratio_setting"
  "github.com/glebarez/sqlite"
  "github.com/stretchr/testify/require"
  "gorm.io/gorm"
)

func TestHarnessPricingGroupOptInAndPriority(t *testing.T) {
 oldDB, oldCache := DB, common.MemoryCacheEnabled
 oldCfg := setting.GetIntelligentRoutingConfig()
 if oldCfg.GlobalChannelOrder == nil { oldCfg.GlobalChannelOrder = []int{} }
 oldRatios := ratio_setting.GroupRatio2JSONString()
 t.Cleanup(func() { require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(oldRatios)) })
 require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"a":1,"b":0.8,"c":0.7}`))
 db, err := gorm.Open(sqlite.Open("file:harness-pricing-test?mode=memory&cache=shared"), &gorm.Config{})
 require.NoError(t,err)
 DB = db
 common.MemoryCacheEnabled = false
 t.Cleanup(func() {
   DB = oldDB
   common.MemoryCacheEnabled = oldCache
   require.NoError(t,setting.UpdateIntelligentRoutingConfig(oldCfg))
   sqlDB,err := db.DB()
   if err == nil { _ = sqlDB.Close() }
 })
 require.NoError(t,db.AutoMigrate(&Channel{},&Ability{}))
 a := Channel{Type:1,Key:"test",Name:"a",Models:"model-x",Group:"a,b",Status:common.ChannelStatusEnabled}
 b := Channel{Type:1,Key:"test",Name:"b",Models:"model-x",Group:"c",Status:common.ChannelStatusEnabled}
 require.NoError(t,db.Create(&a).Error)
 require.NoError(t,db.Create(&b).Error)
 for _, item := range []Ability{
   {Group:"a",Model:"model-x",ChannelId:a.Id,Enabled:true},
   {Group:"b",Model:"model-x",ChannelId:a.Id,Enabled:true},
   {Group:"c",Model:"model-x",ChannelId:b.Id,Enabled:true},
 } { require.NoError(t,db.Create(&item).Error) }
 cfg := setting.IntelligentRoutingConfig{GlobalChannelOrder:[]int{a.Id,b.Id},PricingGroupOrder:[]string{"c","b"}}
 require.NoError(t,setting.UpdateIntelligentRoutingConfig(cfg))
 group,ok,err := GetPreferredPricingGroupForChannelModel(a.Id,"model-x")
 require.NoError(t,err)
 require.True(t,ok)
 require.Equal(t,"b",group)
 chosen,err := GetRandomSatisfiedChannelWithPricingGroups("global","default","model-x",0,[]dto.ChannelFilter{},[]int{a.Id,b.Id})
 require.NoError(t,err)
 require.Equal(t,a.Id,chosen.Id)
 cfg.PricingGroupOrder = []string{"c"}
 require.NoError(t,setting.UpdateIntelligentRoutingConfig(cfg))
 chosen,err = GetRandomSatisfiedChannelWithPricingGroups("global","default","model-x",0,[]dto.ChannelFilter{},[]int{a.Id,b.Id})
 require.NoError(t,err)
 require.Equal(t,b.Id,chosen.Id)
 chosen,err = GetRandomSatisfiedChannelWithPricingGroups("global","default","model-x",1,[]dto.ChannelFilter{},[]int{a.Id,b.Id})
 require.NoError(t,err)
 require.Nil(t,chosen)
 cfg.PricingGroupOrder = []string{}
 require.NoError(t,setting.UpdateIntelligentRoutingConfig(cfg))
 chosen,err = GetRandomSatisfiedChannelWithPricingGroups("global","default","model-x",0,[]dto.ChannelFilter{},[]int{a.Id,b.Id})
 require.NoError(t,err)
 require.Nil(t,chosen)
}
