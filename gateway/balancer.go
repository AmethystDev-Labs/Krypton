package gateway

import (
	"hash/fnv"
	"math"
	"math/rand"
	"net/http/httputil"
	"net/url"
	"sync"
	"sync/atomic"
	"time"
)

type Node struct {
	ID              string
	Address         string
	targetURL       *url.URL
	Proxy           *httputil.ReverseProxy
	InitialWeight   int32
	effectiveWeight int32
	currentWeight   int32
	passiveScore    int32
	activeScore     int32
	checkScript     string
	penaltyWindow   uint64
	inflight        int32
	connDeltaBits   uint64
}

type Bucket struct {
	mu    sync.Mutex
	nodes []*Node
}

type Balancer struct {
	buckets       []*Bucket
	nodeMap       sync.Map
	config        *Config
	rand          *rand.Rand
	randMu        sync.Mutex
	cfgMu         sync.RWMutex
	totalInflight int64
	nodeCount     int32
}

func NewBalancer(cfg *Config) (*Balancer, error) {
	b := &Balancer{
		buckets: make([]*Bucket, cfg.Gateway.Shards),
		config:  cfg,
		rand:    rand.New(rand.NewSource(time.Now().UnixNano())),
	}
	setTransportConfig(cfg)
	for i := 0; i < cfg.Gateway.Shards; i++ {
		b.buckets[i] = &Bucket{}
	}

	for _, nc := range cfg.Nodes {
		node, err := NewNode(nc)
		if err != nil {
			return nil, err
		}
		idx := b.pickBucketIndex(nc.Address)
		b.buckets[idx].nodes = append(b.buckets[idx].nodes, node)
		b.nodeMap.Store(nc.Address, node)
	}
	b.nodeCount = int32(len(cfg.Nodes))
	return b, nil
}

func (b *Balancer) pickBucketIndex(key string) int {
	if len(b.buckets) == 1 {
		return 0
	}
	if b.config.Strategy.HashShard {
		h := fnv.New32a()
		_, _ = h.Write([]byte(key))
		return int(h.Sum32() % uint32(len(b.buckets)))
	}
	b.randMu.Lock()
	defer b.randMu.Unlock()
	return b.rand.Intn(len(b.buckets))
}

func (b *Balancer) Select(key string) *Node {
	b.cfgMu.RLock()
	idx := b.pickBucketIndex(key)
	bucket := b.buckets[idx]
	if len(bucket.nodes) == 0 {
		// fallback: find the first non-empty bucket
		for _, bk := range b.buckets {
			if len(bk.nodes) > 0 {
				bucket = bk
				break
			}
		}
	}
	b.cfgMu.RUnlock()

	bucket.mu.Lock()
	defer bucket.mu.Unlock()

	var total int32
	var best *Node
	for _, n := range bucket.nodes {
		ew := atomic.LoadInt32(&n.effectiveWeight)
		n.currentWeight += ew
		total += ew
		if best == nil || n.currentWeight > best.currentWeight {
			best = n
		}
	}
	if best != nil {
		best.currentWeight -= total
	}
	return best
}

func (b *Balancer) ForEachNode(fn func(n *Node)) {
	for _, bucket := range b.buckets {
		bucket.mu.Lock()
		nodes := append([]*Node(nil), bucket.nodes...)
		bucket.mu.Unlock()
		for _, n := range nodes {
			fn(n)
		}
	}
}

func (n *Node) UpdateEffectiveWeight(delta int32, min int32) {
	for {
		old := atomic.LoadInt32(&n.effectiveWeight)
		newW := old + delta
		if newW < min {
			newW = min
		}
		if newW > n.InitialWeight {
			newW = n.InitialWeight
		}
		if atomic.CompareAndSwapInt32(&n.effectiveWeight, old, newW) {
			return
		}
	}
}

func (n *Node) SyncWeight(passiveScore float64, activeScore float64, connDelta float64) {
	targetScore := math.Min(passiveScore, activeScore)
	if connDelta != 0 {
		targetScore += connDelta
		if targetScore < 0 {
			targetScore = 0
		}
		if targetScore > 100 {
			targetScore = 100
		}
	}
	current := atomic.LoadInt32(&n.effectiveWeight)
	target := int32(float64(n.InitialWeight) * (targetScore / 100.0))
	if target < current {
		atomic.StoreInt32(&n.effectiveWeight, target)
		return
	}
	if target > current {
		step := n.InitialWeight / 20
		next := current + step
		if next > target {
			next = target
		}
		atomic.StoreInt32(&n.effectiveWeight, next)
	}
}

func (n *Node) UpdatePassiveScore(delta int32, maxPenaltyPerSecond int32) {
	if delta < 0 {
		delta = n.limitPenalty(delta, maxPenaltyPerSecond)
		if delta == 0 {
			return
		}
	}
	for {
		old := atomic.LoadInt32(&n.passiveScore)
		next := old + delta
		if next < 0 {
			next = 0
		}
		if next > 100 {
			next = 100
		}
		if atomic.CompareAndSwapInt32(&n.passiveScore, old, next) {
			return
		}
	}
}

func (n *Node) limitPenalty(delta int32, maxPenaltyPerSecond int32) int32 {
	if delta >= 0 || maxPenaltyPerSecond <= 0 {
		return delta
	}
	now := uint64(time.Now().Unix())
	for {
		state := atomic.LoadUint64(&n.penaltyWindow)
		sec := state >> 32
		used := int32(state & 0xffffffff)
		if sec != now {
			sec = now
			used = 0
		}
		remaining := maxPenaltyPerSecond - used
		if remaining <= 0 {
			return 0
		}
		want := -delta
		if want > remaining {
			want = remaining
		}
		newUsed := used + want
		newState := (sec << 32) | uint64(uint32(newUsed))
		if atomic.CompareAndSwapUint64(&n.penaltyWindow, state, newState) {
			return -want
		}
	}
}

func (n *Node) PassiveScore() float64 {
	return float64(atomic.LoadInt32(&n.passiveScore))
}

func (n *Node) SetPassiveScore(score int32) {
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}
	atomic.StoreInt32(&n.passiveScore, score)
}

func (n *Node) SetActiveScore(score int32) {
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}
	atomic.StoreInt32(&n.activeScore, score)
}

func (n *Node) ActiveScore() float64 {
	return float64(atomic.LoadInt32(&n.activeScore))
}

func (n *Node) ConnDelta() float64 {
	return math.Float64frombits(atomic.LoadUint64(&n.connDeltaBits))
}

func (n *Node) SetConnDelta(delta float64) {
	atomic.StoreUint64(&n.connDeltaBits, math.Float64bits(delta))
}

func (b *Balancer) ApplyConfig(next *Config) error {
	b.cfgMu.Lock()
	defer b.cfgMu.Unlock()

	if len(next.Nodes) != len(b.config.Nodes) {
		Warnf("admin reload: node list change ignored (current=%d next=%d)", len(b.config.Nodes), len(next.Nodes))
	}

	b.config.Gateway = next.Gateway
	b.config.Strategy = next.Strategy
	// Node list reload not supported yet; keep current nodes.

	setTransportConfig(b.config)
	b.updateConnFactorLocked()
	return nil
}

func (b *Balancer) adjustConn(node *Node, delta int32) {
	if node == nil || delta == 0 {
		return
	}
	atomic.AddInt32(&node.inflight, delta)
	atomic.AddInt64(&b.totalInflight, int64(delta))
	b.updateConnFactor()
}

func (b *Balancer) updateConnFactor() {
	b.cfgMu.RLock()
	defer b.cfgMu.RUnlock()
	b.updateConnFactorLocked()
}

func (b *Balancer) updateConnFactorLocked() {
	st := b.config.Strategy
	if !st.ConnFactorEnabled {
		b.resetConnFactorLocked()
		return
	}
	nodeCount := float64(b.nodeCount)
	if nodeCount <= 0 {
		return
	}
	total := float64(atomic.LoadInt64(&b.totalInflight))
	smoothing := float64(st.ConnFactorSmoothing)
	slope := st.ConnFactorSlope
	threshold := st.ConnFactorSyncThreshold
	alpha := st.ConnFactorEMAAlpha
	if slope <= 0 {
		slope = 0.4
	}
	if alpha <= 0 || alpha > 1 {
		alpha = 0.2
	}
	if threshold < 0 {
		threshold = 0
	}
	denom := total + smoothing
	targetShare := 1.0 / nodeCount
	b.ForEachNode(func(n *Node) {
		var delta float64
		if denom <= 0 {
			delta = 0
		} else {
			ci := float64(atomic.LoadInt32(&n.inflight))
			share := (ci + (smoothing / nodeCount)) / denom
			delta = clampFloat(10*(targetShare-share)/slope, -10, 10)
		}
		last := n.ConnDelta()
		if math.Abs(delta-last) <= threshold {
			return
		}
		next := last*(1-alpha) + delta*alpha
		n.SetConnDelta(next)
		n.SyncWeight(n.PassiveScore(), n.ActiveScore(), next)
	})
}

func (b *Balancer) resetConnFactorLocked() {
	b.ForEachNode(func(n *Node) {
		if n.ConnDelta() == 0 {
			return
		}
		n.SetConnDelta(0)
		n.SyncWeight(n.PassiveScore(), n.ActiveScore(), 0)
	})
}

func clampFloat(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
