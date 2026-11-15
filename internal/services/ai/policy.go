package ai

import (
	"fmt"
	"sort"
	"strings"
)

// policyEntry is a small, curated FAQ. It is intentionally code (not a vector
// store) in Phase 1: the corpus is tiny, changes with the product, and must be
// reviewable. Phase 3 can move it behind Redis Query Engine or pgvector.
type policyEntry struct {
	Title    string
	Content  string
	Keywords []string
}

var policies = []policyEntry{
	{
		Title: "退货退款流程",
		Content: "订单发货后可以在订单详情或售后页发起退货退款，填写原因、退款金额和退货商品。" +
			"商家审核通过后，按页面提示寄回并填写物流单号；仓库验收后原路退款。" +
			"未发货的订单可以直接取消；已支付但未发货的情况以订单页可用的操作为准。",
		Keywords: []string{"退货", "退款", "售后", "换货", "寄回", "验收"},
	},
	{
		Title: "发货与物流",
		Content: "支付成功后仓库安排发货，物流单号和承运商在订单详情的物流信息中查看。" +
			"秒杀、预售或大促期间发货可能延迟，以订单页展示的状态为准；" +
			"长时间未更新可以联系人工客服核查。",
		Keywords: []string{"发货", "物流", "快递", "什么时候", "到哪", "配送"},
	},
	{
		Title: "订单取消",
		Content: "待支付订单可以在订单详情直接取消，库存会释放。" +
			"已支付订单不能直接取消，需要走退款或售后流程；秒杀订单还受活动规则约束。" +
			"取消后如已扣款，退款按原支付渠道处理。",
		Keywords: []string{"取消", "不想要", "待支付", "未支付"},
	},
	{
		Title: "支付说明",
		Content: "订单支付状态以订单详情为准。支付成功后订单进入待发货；" +
			"如果支付失败或重复支付，请保留支付流水并联系人工客服核对。" +
			"开发环境的模拟支付不代表真实扣款。",
		Keywords: []string{"支付", "付款", "扣款", "重复支付", "流水"},
	},
	{
		Title: "秒杀规则",
		Content: "秒杀需要先预约并获得资格，资格有时效；下单后请在有效期内完成支付，超时订单会关闭并释放库存。" +
			"同一用户和活动通常只能购买限定数量，具体以活动页说明为准。",
		Keywords: []string{"秒杀", "抢购", "资格", "预约", "超时"},
	},
	{
		Title: "修改地址与发票",
		Content: "订单发货前可以尝试联系人工客服修改收货信息；发货后无法修改地址，只能按退货流程处理。" +
			"发票和特殊配送需求请联系人工客服，系统内暂不提供自助开票。",
		Keywords: []string{"地址", "收货信息", "发票", "改地址"},
	},
}

// searchPolicy returns the best-matching entries. Keyword overlap is enough for
// this corpus and keeps the answer grounded in reviewed text.
func searchPolicy(query string, limit int) []policyEntry {
	query = strings.TrimSpace(query)
	if query == "" || limit <= 0 {
		return nil
	}
	type scored struct {
		entry policyEntry
		score int
	}
	var matches []scored
	for _, p := range policies {
		score := 0
		for _, kw := range p.Keywords {
			if strings.Contains(query, kw) || strings.Contains(p.Title, query) {
				score++
			}
		}
		if score > 0 {
			matches = append(matches, scored{entry: p, score: score})
		}
	}
	sort.SliceStable(matches, func(i, j int) bool { return matches[i].score > matches[j].score })
	if len(matches) > limit {
		matches = matches[:limit]
	}
	out := make([]policyEntry, 0, len(matches))
	for _, m := range matches {
		out = append(out, m.entry)
	}
	return out
}

func formatPolicy(entries []policyEntry) (string, []string) {
	if len(entries) == 0 {
		return "没有找到对应的商城规则，建议转人工客服确认。", nil
	}
	var b strings.Builder
	sources := make([]string, 0, len(entries))
	for i, e := range entries {
		if i > 0 {
			b.WriteString("\n\n")
		}
		fmt.Fprintf(&b, "%s：%s", e.Title, e.Content)
		sources = append(sources, e.Title)
	}
	return b.String(), sources
}
