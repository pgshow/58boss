package initial

import (
	"58boss/config"
	"58boss/util"
	"github.com/astaxie/beego/logs"
)

func Init() {
	// 搜索词
	searchTmp := util.GetList("./searchKeywords.txt")
	config.SearKeywords = util.ListDrop(searchTmp)

	// 匹配词
	//matchTmp := util.GetList("./rejectKeywords.txt")
	//for _, m := range matchTmp{
	//	config.RejectKeywords = append(config.RejectKeywords, m)
	//}
	//config.RejectKeywords = util.ListDrop(matchTmp)

	logs.Info("Initial System")
}
