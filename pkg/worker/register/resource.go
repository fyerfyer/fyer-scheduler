package register

import (
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/fyerfyer/fyer-scheduler/pkg/common/utils"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/load"
	"github.com/shirou/gopsutil/v3/mem"
	"go.uber.org/zap"
)

// ResourceCollector 负责收集系统资源信息
type ResourceCollector struct {
	lastResourceInfo ResourceInfo
	lock             sync.RWMutex
	collectInterval  time.Duration
	stopChan         chan struct{}
	isRunning        bool
}

// NewResourceCollector 创建一个新的资源收集器
func NewResourceCollector(interval time.Duration) *ResourceCollector {
	if interval <= 0 {
		interval = 60 * time.Second // 默认60秒收集一次
	}

	return &ResourceCollector{
		lastResourceInfo: ResourceInfo{},
		collectInterval:  interval,
		stopChan:         make(chan struct{}),
		isRunning:        false,
	}
}

// Start 启动资源收集
func (rc *ResourceCollector) Start() {
	rc.lock.Lock()
	defer rc.lock.Unlock()

	if rc.isRunning {
		return
	}

	rc.isRunning = true
	rc.stopChan = make(chan struct{})

	// 立即收集一次资源信息
	rc.lastResourceInfo = CollectSystemResources()

	// 启动定期收集协程
	go rc.collectLoop()

	utils.Info("resource collector started", zap.Duration("interval", rc.collectInterval))
}

// Stop 停止资源收集
func (rc *ResourceCollector) Stop() {
	rc.lock.Lock()
	defer rc.lock.Unlock()

	if !rc.isRunning {
		return
	}

	rc.isRunning = false
	close(rc.stopChan)

	utils.Info("resource collector stopped")
}

// GetResourceInfo 获取最新的资源信息
func (rc *ResourceCollector) GetResourceInfo() ResourceInfo {
	rc.lock.RLock()
	defer rc.lock.RUnlock()

	return rc.lastResourceInfo
}

// collectLoop 定期收集资源信息
func (rc *ResourceCollector) collectLoop() {
	ticker := time.NewTicker(rc.collectInterval)
	defer ticker.Stop()

	for {
		select {
		case <-rc.stopChan:
			return
		case <-ticker.C:
			// 收集资源信息
			resourceInfo := CollectSystemResources()

			// 更新最新资源信息
			rc.lock.Lock()
			rc.lastResourceInfo = resourceInfo
			rc.lock.Unlock()

			utils.Debug("system resources collected",
				zap.Float64("load_avg", resourceInfo.LoadAvg),
				zap.Int64("memory_free_mb", resourceInfo.MemoryFree),
				zap.Int64("disk_free_gb", resourceInfo.DiskFree))
		}
	}
}

// CollectSystemResources 收集系统资源信息
func CollectSystemResources() ResourceInfo {
	resourceInfo := ResourceInfo{
		CPUCores: runtime.NumCPU(), // 获取CPU核心数
	}

	// 收集内存信息
	collectMemoryInfo(&resourceInfo)

	// 收集磁盘信息
	collectDiskInfo(&resourceInfo)

	// 收集系统负载信息
	collectLoadInfo(&resourceInfo)

	return resourceInfo
}

// collectMemoryInfo 收集内存信息
func collectMemoryInfo(info *ResourceInfo) {
	memInfo, err := mem.VirtualMemory()
	if err != nil {
		utils.Error("failed to collect memory info", zap.Error(err))
		// 设置默认值
		info.MemoryTotal = 0
		info.MemoryFree = 0
		return
	}

	// 转换为MB
	info.MemoryTotal = int64(memInfo.Total / 1024 / 1024)
	info.MemoryFree = int64(memInfo.Available / 1024 / 1024)
}

// collectDiskInfo 收集磁盘信息
func collectDiskInfo(info *ResourceInfo) {
	// 获取根目录的磁盘使用情况
	path := "/"
	if runtime.GOOS == "windows" {
		path = "C:\\"
	}

	diskStat, err := disk.Usage(path)
	if err != nil {
		utils.Error("failed to collect disk info",
			zap.String("path", path),
			zap.Error(err))
		// 设置默认值
		info.DiskTotal = 0
		info.DiskFree = 0
		return
	}

	// 转换为GB
	info.DiskTotal = int64(diskStat.Total / 1024 / 1024 / 1024)
	info.DiskFree = int64(diskStat.Free / 1024 / 1024 / 1024)

	// 如果是0，设置为默认值防止除零错误
	if info.DiskTotal == 0 {
		info.DiskTotal = 1
	}
	if info.DiskFree == 0 {
		info.DiskFree = 1
	}
}

// collectLoadInfo 收集系统负载信息
func collectLoadInfo(info *ResourceInfo) {
	// 获取系统负载
	loadInfo, err := load.Avg()
	if err != nil {
		utils.Error("failed to collect load info", zap.Error(err))
		// 设置默认值
		info.LoadAvg = 0.0
		return
	}

	// 使用1分钟负载
	info.LoadAvg = loadInfo.Load1
}

// GetCPUUsage 获取CPU使用率
func GetCPUUsage() (float64, error) {
	percentage, err := cpu.Percent(1*time.Second, false)
	if err != nil {
		return 0.0, fmt.Errorf("failed to get CPU usage: %w", err)
	}

	if len(percentage) == 0 {
		return 0.0, fmt.Errorf("no CPU usage data available")
	}

	return percentage[0], nil
}

// GetMemoryUsagePercent 获取内存使用率
func GetMemoryUsagePercent() (float64, error) {
	memInfo, err := mem.VirtualMemory()
	if err != nil {
		return 0.0, fmt.Errorf("failed to get memory info: %w", err)
	}

	return memInfo.UsedPercent, nil
}

// GetDiskUsagePercent 获取磁盘使用率
func GetDiskUsagePercent(path string) (float64, error) {
	if path == "" {
		path = "/"
		if runtime.GOOS == "windows" {
			path = "C:\\"
		}
	}

	diskStat, err := disk.Usage(path)
	if err != nil {
		return 0.0, fmt.Errorf("failed to get disk usage for path %s: %w", path, err)
	}

	return diskStat.UsedPercent, nil
}
