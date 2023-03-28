package ihelfu

//type iheEvict struct {
//	// 记录key以及访问次数
//	// TODO 以cms替换
//	accessCounts map[string]int
//
//	totalSize int
//	outerCh   chan string
//
//	wz  *winZone
//	edz *edenZone
//	evz *evictZone
//}
//
//func (i *iheEvict) Evict() {
//	var key string
//	for key = range i.outerCh {
//
//	}
//}
//
//type winZone struct {
//	winLimit int
//	winSafe  int
//	winTimer time.Ticker
//	windows  *list.List
//}
//
//func (w *winZone) cleanPeriodicity() {
//	for range w.winTimer.C {
//		if w.windows.Len() > w.winSafe {
//
//		}
//	}
//}
//
//type edenZone struct {
//	edenLimit int
//	edenSafe  int
//	edenTimer time.Ticker
//	eden      *list.List
//}
//
//type evictZone struct {
//	evictLimit int
//	evict      []string
//}
//
//func (e evictZone) Insert(key string) {
//	if len(e.evict) >= e.evictLimit {
//		e.clear()
//	}
//	e.evict = append(e.evict, key)
//}
//
//func (e *evictZone) clear() {
//	e.evict = make([]string, 0, e.evictLimit)
//}
//
//var (
//	winRatio  = 0.2
//	edenRatio = 0.7
//	evict     = 0.1
//
//	winSafeRatio  = 0.5
//	edenSafeRatio = 0.14
//)
//
//func NewIheEvict(sizeLimit int) *iheEvict {
//
//}
