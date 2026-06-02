package main

import (
	"context"
	"sort"
	"sync"
	"time"

	xcontrol "github.com/75912001/xlib/control"
	xevent "github.com/75912001/xlib/event"
	xtimer "github.com/75912001/xlib/timer"
)

var GRobotManager *RobotManager

type RobotManager struct {
	iEventMgr   *xevent.ListMgr
	robotsByUID map[uint64]*Robot
	robots      []*Robot
	stats       *RobotStats
	mu          sync.RWMutex
	closeOnce   sync.Once
	closed      chan struct{}
	summary     *xtimer.Millisecond
}

func NewRobotManager() *RobotManager {
	return &RobotManager{
		iEventMgr:   xevent.NewListMgr(1, Bus),
		robotsByUID: make(map[uint64]*Robot),
		stats:       &RobotStats{},
		closed:      make(chan struct{}),
	}
}

func (p *RobotManager) StartEventLoop() {
	p.iEventMgr.Start()
}

func (p *RobotManager) Start(ctx context.Context) error {
	p.buildRobots()
	p.startSummaryTimer()
	go p.startRobots(ctx)
	return nil
}

func (p *RobotManager) buildRobots() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.robots) != 0 {
		return
	}
	cfg := GConfigYaml.Robot
	for i := 0; i < cfg.Count; i++ {
		uid := cfg.UIDStart + uint64(i)*cfg.UIDStep
		robot := NewRobot(p, uid)
		p.robots = append(p.robots, robot)
		p.robotsByUID[uid] = robot
	}
}

func (p *RobotManager) startRobots(ctx context.Context) {
	if _, err := waitGatewayAddr(5 * time.Second); err != nil {
		ColorPrintf(Red, "wait gateway failed: %v\n", err)
		log.Errorf("wait gateway failed: %v", err)
		p.stats.commandError.Add(1)
		return
	}
	cfg := GConfigYaml.Robot
	robots := p.Robots()
	for i, robot := range robots {
		if p.isClosed() {
			return
		}
		go func(r *Robot) {
			if err := r.Start(ctx); err != nil {
				p.stats.connectFail.Add(1)
				ColorPrintf(Red, "robot start failed uid=%d err=%v\n", r.uid, err)
				log.Errorf("robot start failed uid=%d err=%v", r.uid, err)
			}
		}(robot)
		if (i+1)%cfg.StartupBatchSize == 0 {
			time.Sleep(cfg.StartupBatchInterval)
		}
	}
}

func (p *RobotManager) Robots() []*Robot {
	p.mu.RLock()
	defer p.mu.RUnlock()
	robots := append([]*Robot(nil), p.robots...)
	sort.Slice(robots, func(i, j int) bool {
		return robots[i].uid < robots[j].uid
	})
	return robots
}

func (p *RobotManager) Find(uid uint64) (*Robot, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	robot, ok := p.robotsByUID[uid]
	return robot, ok
}

func (p *RobotManager) Total() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.robots)
}

func (p *RobotManager) Stop() {
	p.closeOnce.Do(func() {
		for _, robot := range p.Robots() {
			robot.Close()
		}
		if xtimer.GTimer != nil && p.summary != nil {
			xtimer.GTimer.DelMillisecond(p.summary)
			p.summary = nil
		}
		if p.iEventMgr != nil {
			p.iEventMgr.Stop()
		}
		close(p.closed)
	})
}

func (p *RobotManager) isClosed() bool {
	select {
	case <-p.closed:
		return true
	default:
		return false
	}
}

func (p *RobotManager) startSummaryTimer() {
	if xtimer.GTimer == nil {
		return
	}
	interval := GConfigYaml.Robot.Logging.SummaryInterval
	if interval <= 0 {
		return
	}
	cb := xcontrol.NewCallBack(func(args ...any) error {
		p.summary = nil
		if p.isClosed() {
			return nil
		}
		p.PrintStats()
		p.startSummaryTimer()
		return nil
	})
	p.summary = xtimer.GTimer.AddMillisecond(cb, time.Now().Add(interval).UnixMilli(), p.iEventMgr)
}

func (p *RobotManager) PrintStats() {
	text := p.stats.Snapshot().String(p.Total())
	ColorPrintf(Cyan, "%s\n", text)
	log.Infof("robot stats %s", text)
}

func (p *RobotManager) RobotViews(limit int) []robotView {
	robots := p.Robots()
	if limit > 0 && len(robots) > limit {
		robots = robots[:limit]
	}
	out := make([]robotView, 0, len(robots))
	for _, robot := range robots {
		out = append(out, robot.View())
	}
	return out
}
