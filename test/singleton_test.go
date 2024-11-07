package test

import (
	"sync"
	"testing"
)

type Singleton struct{}

// sync.Once 确保只执行一次
var instance *Singleton
var once sync.Once

// GetInstance 返回单例对象
func GetInstance() *Singleton {
	once.Do(func() {
		instance = &Singleton{}
	})
	return instance
}

func TestGetInstance(t *testing.T) {
	// 获取第一个实例
	instance1 := GetInstance()
	// 获取第二个实例
	instance2 := GetInstance()

	// 判断两个实例是否相同
	if instance1 != instance2 {
		t.Errorf("GetInstance() returned different instances")
	} else {
		t.Logf("GetInstance() returned same instance")
	}
}

func TestSingletonConcurrency(t *testing.T) {
	var instance1, instance2 *Singleton
	var wg sync.WaitGroup

	// 使用 WaitGroup 等待两个 goroutine 都执行完
	wg.Add(2)

	// 第一个 goroutine 获取实例
	go func() {
		defer wg.Done()
		instance1 = GetInstance()
	}()

	// 第二个 goroutine 获取实例
	go func() {
		defer wg.Done()
		instance2 = GetInstance()
	}()

	// 等待两个 goroutine 都完成
	wg.Wait()

	// 判断两个实例是否相同
	if instance1 != instance2 {
		t.Errorf("GetInstance() returned different instances in concurrent execution")
	} else {
		t.Logf("GetInstance() returned same instance")
	}
}
