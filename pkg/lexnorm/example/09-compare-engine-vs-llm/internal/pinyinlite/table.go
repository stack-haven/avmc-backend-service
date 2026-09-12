// Package pinyinlite 提供一个最小化的汉字 → 拼音映射表，
// 专门覆盖本 example 中词库 + 测试文本出现的所有汉字（363 个）。
//
// # 范围与局限
//
//   - 仅覆盖本 example 出现的汉字，未覆盖的字返回 ("", false)
//   - 多音字按本 example 上下文的最常见读音选择（如"行"→xing、"长"→chang）
//   - 不处理轻声、儿化音
//   - 整词拼音生成时用空格分隔每个字
//
// # 设计动机
//
// ark-lexnorm 要求零第三方依赖。本包不引入任何外部拼音库，
// 仅维护一个紧凑的本地映射表，足以驱动同音字索引与模糊匹配。
package pinyinlite

import "strings"

// 紧凑表：所有汉字按 Unicode 升序拼接成一个 rune 序列，
// 对应位置的拼音用 | 分隔（同 table 顺序）。
//
// 注意：必须保证 hzTable 的字符顺序与 pyTable 的拼音顺序严格对应。
const hzTable = "一七万三上下业丝个中主丽之乖书事二于云五些交产人仔件份伍会传体何佘作你供依保修倍做儿光克入八六关兴其军冯冰决准凤出分划利到前力加务动助劳化医十千升午华协卜卡卢历原参及友发取可史叶号各合同向君吴呃告员咯品哈哒哥商喵嘻四团城填处备夏外多大奇好娃婷嫣子孙孟季学宇安完定实客家寥对小屁岩川巡工布帮并广应康建开张录徐德快态思性总情成户打执扬技抓投招持按据排接推提播效数整料新早旭明春是晓智月有未术朱权李杜条来杨松板析林架查标根格框桑梓梦模正段求江沆沟没河流测海清渡温游源潮热焓照熊燕片物独王珠理瓦甫田瘤百的盛相真确社种科程穿立童端第等策管精级线组细结给统续综缝羊群考者职肖肯肿胜能航芦花莲菌萍萧落行表袁西要视解计认议设试话询语说课豪账费贺资起路践转软辉进通速遇邓部郭配野金鑫锡门问阳阶阿陈限院隔集青静靠面页项须频颗题风飞饭驶驾高黑鼎龙龚"

const pyTable = "yi|qi|wan|san|shang|xia|ye|si|ge|zhong|zhu|li|zhi|guai|shu|shi|er|yu|yun|wu|xie|jiao|chan|ren|zai|jian|fen|wu|hui|chuan|ti|he|she|zuo|ni|gong|yi|bao|xiu|bei|zuo|er|guang|ke|ru|ba|liu|guan|xing|qi|jun|feng|bing|jue|zhun|feng|chu|fen|hua|li|dao|qian|li|jia|wu|dong|zhu|lao|hua|yi|shi|qian|sheng|wu|hua|xie|bu|qia|lu|li|yuan|can|ji|you|fa|qu|ke|shi|ye|hao|ge|he|tong|xiang|jun|wu|e|gao|yuan|ge|pin|ha|da|ge|shang|miao|xi|si|tuan|cheng|tian|chu|bei|xia|wai|duo|da|qi|hao|wa|ting|yan|zi|sun|meng|ji|xue|yu|an|wan|ding|shi|ke|jia|liao|dui|xiao|pi|yan|chuan|xun|gong|bu|bang|bing|guang|ying|kang|jian|kai|zhang|lu|xu|de|kuai|tai|si|xing|zong|qing|cheng|hu|da|zhi|yang|ji|zhua|tou|zhao|chi|an|ju|pai|jie|tui|ti|bo|xiao|shu|zheng|liang|xin|zao|xu|ming|chun|shi|xiao|zhi|yue|you|wei|shu|zhu|quan|li|du|tiao|lai|yang|song|ban|xi|lin|jia|cha|biao|gen|ge|kuang|sang|zi|meng|mo|zheng|duan|qiu|jiang|huang|gou|mei|he|liu|ce|hai|qing|du|wen|you|yuan|chao|re|han|zhao|xiong|yan|pian|wu|du|wang|zhu|li|wa|fu|tian|liu|bai|de|sheng|xiang|zhen|que|she|zhong|ke|cheng|chuan|li|tong|duan|di|deng|ce|guan|jing|ji|xian|zu|xi|jie|gei|tong|xu|zong|feng|yang|qun|kao|zhe|zhi|xiao|ken|zhong|sheng|neng|hang|lu|hua|lian|jun|ping|xiao|luo|xing|biao|yuan|xi|yao|shi|jie|ji|ren|yi|she|shi|hua|xun|yu|shuo|ke|hao|zhang|fei|he|zi|qi|lu|jian|zhuan|ruan|hui|jin|tong|su|yu|deng|bu|guo|pei|ye|jin|xin|xi|men|wen|yang|jie|a|chen|xian|yuan|ge|ji|qing|jing|kao|mian|ye|xiang|xu|pin|ke|ti|feng|fei|fan|shi|jia|gao|hei|ding|long|gong"

// 内部表：构建一次
var pyMap map[rune]string

func init() {
	hzRunes := []rune(hzTable)
	pyParts := strings.Split(pyTable, "|")
	if len(hzRunes) != len(pyParts) {
		panic("pinyinlite: hz/py table length mismatch")
	}
	pyMap = make(map[rune]string, len(hzRunes))
	for i, r := range hzRunes {
		pyMap[r] = pyParts[i]
	}
}

// Lookup 返回单字拼音。
//
// 第二个返回值表示是否命中本表。未命中时返回 ("", false)，
// 调用方应跳过该字的拼音（不要 fallback 别的音）。
func Lookup(r rune) (string, bool) {
	if p, ok := pyMap[r]; ok {
		return p, true
	}
	return "", false
}

// Convert 返回整串文本的拼音（空格分隔，每个汉字对应一个拼音）。
//
// 未覆盖的汉字会被跳过（不输出占位符），便于调用方判断覆盖完整度。
func Convert(s string) string {
	var b strings.Builder
	for _, r := range s {
		if p, ok := pyMap[r]; ok {
			if b.Len() > 0 {
				b.WriteByte(' ')
			}
			b.WriteString(p)
		}
	}
	return b.String()
}

// Coverage 报告本表对给定字符串中汉字的覆盖率（命中数 / 汉字总数）。
// 用于诊断 transform 阶段是否需要扩充表。
func Coverage(s string) (hit, total int) {
	for _, r := range s {
		if r < 0x4e00 || r > 0x9fff {
			continue
		}
		total++
		if _, ok := pyMap[r]; ok {
			hit++
		}
	}
	return
}
