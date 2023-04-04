package model

import (
	"encoding/json"
	"fmt"
	"github.com/redis/go-redis/v9"
	"log"
	"server/config"
	"server/plugin/db"
	"strconv"
	"strings"
	"time"
)

// Movie 影片基本信息
type Movie struct {
	Id       int64  `json:"id"`       // 影片ID
	Name     string `json:"name"`     // 影片名
	Cid      int64  `json:"cid"`      // 所属分类ID
	CName    string `json:"CName"`    // 所属分类名称
	EnName   string `json:"enName"`   // 英文片名
	Time     string `json:"time"`     // 更新时间
	Remarks  string `json:"remarks"`  // 备注 | 清晰度
	PlayFrom string `json:"playFrom"` // 播放来源
}

// MovieDescriptor 影片详情介绍信息
type MovieDescriptor struct {
	SubTitle    string `json:"subTitle"`    //子标题
	CName       string `json:"cName"`       //分类名称
	EnName      string `json:"enName"`      //英文名
	Initial     string `json:"initial"`     //首字母
	ClassTag    string `json:"classTag"`    //分类标签
	Actor       string `json:"actor"`       //主演
	Director    string `json:"director"`    //导演
	Writer      string `json:"writer"`      //作者
	Blurb       string `json:"blurb"`       //简介, 残缺,不建议使用
	Remarks     string `json:"remarks"`     // 更新情况
	ReleaseDate string `json:"releaseDate"` //上映时间
	Area        string `json:"area"`        // 地区
	Language    string `json:"language"`    //语言
	Year        string `json:"year"`        //年份
	State       string `json:"state"`       //影片状态 正片|预告...
	UpdateTime  string `json:"updateTime"`  //更新时间
	AddTime     int64  `json:"addTime"`     //资源添加时间戳
	DbId        int64  `json:"dbId"`        //豆瓣id
	DbScore     string `json:"dbScore"`     // 豆瓣评分
	Content     string `json:"content"`     //内容简介
}

// MovieBasicInfo 影片基本信息
type MovieBasicInfo struct {
	Id       int64  `json:"id"`       //影片Id
	Cid      int64  `json:"cid"`      //分类ID
	Pid      int64  `json:"pid"`      //一级分类ID
	Name     string `json:"name"`     //片名
	SubTitle string `json:"subTitle"` //子标题
	CName    string `json:"cName"`    //分类名称
	State    string `json:"state"`    //影片状态 正片|预告...
	Picture  string `json:"picture"`  //简介图片
	Actor    string `json:"actor"`    //主演
	Director string `json:"director"` //导演
	Blurb    string `json:"blurb"`    //简介, 不完整
	Remarks  string `json:"remarks"`  // 更新情况
	Area     string `json:"area"`     // 地区
	Year     string `json:"year"`     //年份
}

// MovieUrlInfo 影视资源url信息
type MovieUrlInfo struct {
	Episode string `json:"episode"` // 集数
	Link    string `json:"link"`    // 播放地址
}

// MovieDetail 影片详情信息
type MovieDetail struct {
	Id       int64    `json:"id"`       //影片Id
	Cid      int64    `json:"cid"`      //分类ID
	Pid      int64    `json:"pid"`      //一级分类ID
	Name     string   `json:"name"`     //片名
	Picture  string   `json:"picture"`  //简介图片
	PlayFrom []string `json:"playFrom"` // 播放来源
	DownFrom string   `json:"DownFrom"` //下载来源 例: http
	//PlaySeparator   string              `json:"playSeparator"` // 播放信息分隔符
	PlayList        [][]MovieUrlInfo    `json:"playList"`     //播放地址url
	DownloadList    [][]MovieUrlInfo    `json:"downloadList"` // 下载url地址
	MovieDescriptor `json:"descriptor"` //影片描述信息
}

// SaveMoves  保存影片分页请求list
func SaveMoves(list []Movie) (err error) {
	// 整合数据
	for _, m := range list {
		//score, _ := time.ParseInLocation(time.DateTime, m.Time, time.Local)
		movie, _ := json.Marshal(m)
		// 以Cid为目录为集合进行存储, 便于后续搜索, 以影片id为分值进行存储 例 MovieList:Cid%d
		err = db.Rdb.ZAdd(db.Cxt, fmt.Sprintf(config.MovieListInfoKey, m.Cid), redis.Z{Score: float64(m.Id), Member: movie}).Err()
	}
	return err
}

// AllMovieInfoKey 获取redis中所有的影视列表信息key MovieList:Cid
func AllMovieInfoKey() []string {
	return db.Rdb.Keys(db.Cxt, fmt.Sprint("MovieList:Cid*")).Val()
}

// GetMovieListByKey 获取指定分类的影片列表数据
func GetMovieListByKey(key string) []string {
	return db.Rdb.ZRange(db.Cxt, key, 0, -1).Val()
}

// SaveDetails 保存影片详情信息到redis中 格式: MovieDetail:Cid?:Id?
func SaveDetails(list []MovieDetail) (err error) {
	// 遍历list中的信息
	for _, detail := range list {
		// 序列化影片详情信息
		data, _ := json.Marshal(detail)
		// 1. 原使用Zset存储, 但是不便于单个检索 db.Rdb.ZAdd(db.Cxt, fmt.Sprintf("%s:Cid%d", config.MovieDetailKey, detail.Cid), redis.Z{Score: float64(detail.Id), Member: member}).Err()
		// 改为普通 k v 存储, k-> id关键字, v json序列化的结果, //只保留十天, 没周更新一次
		err = db.Rdb.Set(db.Cxt, fmt.Sprintf(config.MovieDetailKey, detail.Cid, detail.Id), data, config.CategoryTreeExpired).Err()
		// 2. 同步保存简略信息到redis中
		SaveMovieBasicInfo(detail)
		// 3. 保存Search检索信息到redis
		if err == nil {
			// 转换 detail信息
			searchInfo := ConvertSearchInfo(detail)
			// 放弃redis进行检索, 多条件处理不方便
			//err = AddSearchInfo(searchInfo)
			// 只存储用于检索对应影片的关键字信息
			SearchKeyword(searchInfo)
		}

	}
	// 保存一份search信息到mysql, 批量存储
	BatchSaveSearchInfo(list)
	return err
}

// SaveMovieBasicInfo 摘取影片的详情部分信息转存为影视基本信息
func SaveMovieBasicInfo(detail MovieDetail) {
	basicInfo := MovieBasicInfo{
		Id:       detail.Id,
		Cid:      detail.Cid,
		Pid:      detail.Pid,
		Name:     detail.Name,
		SubTitle: detail.SubTitle,
		CName:    detail.CName,
		State:    detail.State,
		Picture:  detail.Picture,
		Actor:    detail.Actor,
		Director: detail.Director,
		Blurb:    detail.Blurb,
		Remarks:  detail.Remarks,
		Area:     detail.Area,
		Year:     detail.Year,
	}
	data, _ := json.Marshal(basicInfo)
	_ = db.Rdb.Set(db.Cxt, fmt.Sprintf(config.MovieBasicInfoKey, detail.Cid, detail.Id), data, config.CategoryTreeExpired).Err()
}

// AddSearchInfo 将影片关键字信息整合后存入search 集合中
func AddSearchInfo(searchInfo SearchInfo) (err error) {
	// 片名 Name 分类 CName 类别标签 classTag 地区 Area 语言 Language 年份 Year 首字母 Initial, 排序
	data, _ := json.Marshal(searchInfo)
	// 时间排序 score -->时间戳 DbId 排序 --> 热度, 评分排序 DbScore
	err = db.Rdb.ZAdd(db.Cxt, fmt.Sprintf("%s:Pid%d", config.SearchTimeListKey, searchInfo.Pid), redis.Z{Score: float64(searchInfo.Time), Member: data}).Err()
	err = db.Rdb.ZAdd(db.Cxt, fmt.Sprintf("%s:Pid%d", config.SearchScoreListKey, searchInfo.Pid), redis.Z{Score: searchInfo.Score, Member: data}).Err()
	err = db.Rdb.ZAdd(db.Cxt, fmt.Sprintf("%s:Pid%d", config.SearchHeatListKey, searchInfo.Pid), redis.Z{Score: float64(searchInfo.Rank), Member: data}).Err()
	// 添加搜索关键字信息
	SearchKeyword(searchInfo)
	return
}

// SearchKeyword 设置search关键字集合
func SearchKeyword(search SearchInfo) {
	// 首先获取redis中的search 关键字信息
	key := fmt.Sprintf("%s:Pid%d", config.SearchKeys, search.Pid)
	keyword := db.Rdb.HGetAll(db.Cxt, key).Val()
	if keyword["Year"] == "" {
		currentYear := time.Now().Year()
		year := ""
		for i := 0; i < 12; i++ {
			// 提供当前年份前推十二年的搜索
			year = fmt.Sprintf("%s,%d", year, currentYear-i)
		}
		initial := ""
		for i := 65; i <= 90; i++ {
			initial = fmt.Sprintf("%s,%c", initial, i)
		}
		keyword = map[string]string{
			//"Name":     "",
			"Category": "",
			"Tag":      "",
			"Area":     "",
			"Language": "",
			"Year":     strings.Trim(year, ","),
			"Initial":  strings.Trim(initial, ","),
			"Sort":     "Time,Db,Score", // 默认,一般不修改
		}
	}
	// 分类标签处理
	if !strings.Contains(keyword["Category"], search.CName) {
		keyword["Category"] = strings.Trim(fmt.Sprintf("%s,%s", keyword["Category"], search.CName), ",")
	}
	// 影视内容分类处理
	if strings.Contains(search.ClassTag, "/") {
		for _, t := range strings.Split(search.ClassTag, "/") {
			if !strings.Contains(keyword["Tag"], t) {
				keyword["Tag"] = strings.Trim(fmt.Sprintf("%s,%s", keyword["Tag"], t), ",")
			}
		}
	} else if strings.Contains(search.ClassTag, ",") {
		for _, t := range strings.Split(search.ClassTag, ",") {
			if !strings.Contains(keyword["Tag"], t) {
				keyword["Tag"] = strings.Trim(fmt.Sprintf("%s,%s", keyword["Tag"], t), ",")
			}
		}
	} else {
		if !strings.Contains(keyword["Tag"], search.ClassTag) {
			keyword["Tag"] = strings.Trim(fmt.Sprintf("%s,%s", keyword["Tag"], search.ClassTag), ",")
		}
	}
	// 如果地区中包含 / 分隔符 则先进行切分处理
	if strings.Contains(search.Area, "/") {
		for _, s := range strings.Split(search.Area, "/") {
			if !strings.Contains(keyword["Area"], strings.TrimSpace(s)) {
				keyword["Area"] = strings.Trim(fmt.Sprintf("%s,%s", keyword["Area"], s), ",")
			}
		}
	} else if strings.Contains(search.Area, ",") {
		for _, s := range strings.Split(search.Area, ",") {
			if !strings.Contains(keyword["Area"], strings.TrimSpace(s)) {
				keyword["Area"] = strings.Trim(fmt.Sprintf("%s,%s", keyword["Area"], s), ",")
			}
		}
	} else {
		if !strings.Contains(keyword["Area"], search.Area) {
			keyword["Area"] = strings.Trim(fmt.Sprintf("%s,%s", keyword["Area"], search.Area), ",")
		}
	}
	// 语言处理
	if strings.Contains(search.Language, "/") {
		for _, l := range strings.Split(search.Language, "/") {
			if !strings.Contains(keyword["Language"], l) {
				keyword["Language"] = strings.Trim(fmt.Sprintf("%s,%s", keyword["Language"], l), ",")
			}
		}

	} else if strings.Contains(search.Language, ",") {
		for _, l := range strings.Split(search.Language, ",") {
			if !strings.Contains(keyword["Language"], l) {
				keyword["Language"] = strings.Trim(fmt.Sprintf("%s,%s", keyword["Language"], l), ",")
			}
		}
	} else {
		if !strings.Contains(keyword["Language"], search.Language) {
			keyword["Language"] = strings.Trim(fmt.Sprintf("%s,%s", keyword["Language"], search.Language), ",")
		}
	}
	_ = db.Rdb.HMSet(db.Cxt, key, keyword).Err()
}

// BatchSaveSearchInfo 批量保存Search信息
func BatchSaveSearchInfo(list []MovieDetail) {
	var infoList []SearchInfo
	for _, v := range list {
		infoList = append(infoList, ConvertSearchInfo(v))
	}
	// 将检索信息存入redis中做一次转存
	RdbSaveSearchInfo(infoList)

	// 废弃方案, 频繁大量入库容易引起主键冲突, 事务影响速率
	// 批量插入时应对已存在数据进行检测, 使用mysql事务进行锁表
	//BatchSave(infoList)
	// 使用批量添加or更新
	//BatchSaveOrUpdate(infoList)
}

// ConvertSearchInfo 将detail信息处理成 searchInfo
func ConvertSearchInfo(detail MovieDetail) SearchInfo {
	score, _ := strconv.ParseFloat(detail.DbScore, 64)
	stamp, _ := time.ParseInLocation(time.DateTime, detail.UpdateTime, time.Local)
	year, err := strconv.ParseInt(detail.Year, 10, 64)
	if err != nil {
		year = 0
	}

	return SearchInfo{
		Mid:      detail.Id,
		Cid:      detail.Cid,
		Pid:      detail.Pid,
		Name:     detail.Name,
		CName:    detail.CName,
		ClassTag: detail.ClassTag,
		Area:     detail.Area,
		Language: detail.Language,
		Year:     year,
		Initial:  detail.Initial,
		Score:    score,
		Rank:     detail.DbId,
		Time:     stamp.Unix(),
		State:    detail.State,
		Remarks:  detail.Remarks,
		// releaseDate 部分影片缺失该参数, 所以使用添加时间作为上映时间排序
		ReleaseDate: detail.AddTime,
	}
}

// GetBasicInfoByKey 获取Id对应的影片基本信息
func GetBasicInfoByKey(key string) MovieBasicInfo {
	// 反序列化得到的结果
	data := []byte(db.Rdb.Get(db.Cxt, key).Val())
	basic := MovieBasicInfo{}
	_ = json.Unmarshal(data, &basic)
	return basic
}

// GetDetailByKey 获取影片对应的详情信息
func GetDetailByKey(key string) MovieDetail {
	// 反序列化得到的结果
	data := []byte(db.Rdb.Get(db.Cxt, key).Val())
	detail := MovieDetail{}
	_ = json.Unmarshal(data, &detail)
	return detail
}

// SearchMovie 搜索关键字影片
func SearchMovie() {
	data, err := db.Rdb.ZScan(db.Cxt, "MovieList:cid30", 0, `*天使*`, config.SearchCount).Val()
	log.Println(err)
	fmt.Println(data)
}