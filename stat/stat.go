package stat

import (
	"runtime"
	"strconv"
	"time"

	"goreplay/config"
	"goreplay/logger"
)

// GorStat stat of gor
type GorStat struct {
	statName string
	rateMs   int
	latest   int
	mean     int
	max      int
	count    int
}

// NewGorStat news a GorStat instance
func NewGorStat(statName string, rateMs int) (s *GorStat) {
	s = new(GorStat)
	s.statName = statName
	s.rateMs = rateMs
	s.latest = 0
	s.mean = 0
	s.max = 0
	s.count = 0

	if config.Settings.Stats {
		go s.reportStats()
	}
	return
}

// Write updates the GorStat
func (s *GorStat) Write(latest int) {
	if config.Settings.Stats {
		if latest > s.max {
			s.max = latest
		}
		if latest != 0 {
			s.mean = ((s.mean * s.count) + latest) / (s.count + 1)
		}
		s.latest = latest
		s.count = s.count + 1
	}
}

// Reset resets the GorStat
func (s *GorStat) Reset() {
	s.latest = 0
	s.max = 0
	s.mean = 0
	s.count = 0
}

// String toString() of GorStat
func (s *GorStat) String() string {
	return s.statName + ":" + strconv.Itoa(s.latest) + "," + strconv.Itoa(s.mean) + "," +
		strconv.Itoa(s.max) + "," + strconv.Itoa(s.count) + "," +
		strconv.Itoa(s.count/(s.rateMs/1000.0)) + "," + strconv.Itoa(runtime.NumGoroutine())
}

// reportStats prints the GorStat
func (s *GorStat) reportStats() {
	logger.Info("\n", s.statName+":latest,mean,max,count,count/second,gcount")
	for {
		logger.Info("\n", s)
		s.Reset()
		time.Sleep(time.Duration(s.rateMs) * time.Millisecond)
	}
}
