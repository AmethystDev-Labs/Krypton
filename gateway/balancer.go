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
}

type Bucket struct {
	mu    sync.Mutex
	nodes []*Node
}

type Balancer struct {
	buckets []*Bucket
	nodeMap sync.Map
	config  *Config
	rand    *rand.Rand
	randMu  sync.Mutex
	cfgMu   sync.RWMutex
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

func (n *Node) SyncWeight(passiveScore float64, activeScore float64) {
	targetScore := math.Min(passiveScore, activeScore)
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

func (n *Node) UpdatePassiveScore(delta int32) {
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
	return nil
}
