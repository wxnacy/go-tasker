// Package godler  provides ...
package tasker

import (
	"fmt"
	"sync"

	"github.com/cheggaaa/pb/v3"
)

type TaskFunc func(*Task) error

type ITasker interface {
	Build() error
	BuildTasks() error
	AddTask(*Task)
	GetTasks() []*Task
	Run(TaskFunc) error
	SyncRun(TaskFunc) error
	AfterRun() error
	BeforeRun() error
}

type Task struct {
	RetryTime int         `json:"retry_count"`
	Err       error       `json:"err"`
	Info      interface{} `json:"info"`
}

type TaskerConfig struct {
	ProcessNum     int
	RetryMaxTime   int
	UseProgressBar bool
}

func NewTaskerConfig() *TaskerConfig {
	return &TaskerConfig{
		ProcessNum:     20,
		RetryMaxTime:   1000,
		UseProgressBar: true,
	}
}

func NewTasker() *Tasker {
	config := NewTaskerConfig()
	return &Tasker{
		Config:      config,
		processChan: make(chan bool, config.ProcessNum),
		resultChan:  make(chan *Task),
		Tasks:       make([]*Task, 0),
	}
}

type Tasker struct {
	TaskId      string
	Config      *TaskerConfig
	Tasks       []*Task
	processChan chan bool
	resultChan  chan *Task
	WaitGroup   sync.WaitGroup
}

func (t *Tasker) BuildTasks() error { return nil }

func (t *Tasker) Build() error { return nil }

func (t *Tasker) AfterRun() error { return nil }

func (t *Tasker) BeforeRun() error { return nil }

func (t *Tasker) RunTask(task *Task) error { return nil }

func (t *Tasker) AddTask(task *Task) {
	t.Tasks = append(t.Tasks, task)
}

func (t *Tasker) GetTasks() []*Task {
	return t.Tasks
}

func (t *Tasker) asyncRunTask(runTaskFunc TaskFunc, task *Task) {
	t.WaitGroup.Add(1)
	go func(task *Task) {
		defer t.WaitGroup.Done()
		t.processChan <- true
		err := runTaskFunc(task)
		// fmt.Println(err)
		task.Err = err
		<-t.processChan
		t.resultChan <- task
	}(task)
}

func (t *Tasker) Run(runTaskFunc TaskFunc) error {

	for _, task := range t.Tasks {
		t.asyncRunTask(runTaskFunc, task)
	}

	go func() {
		t.WaitGroup.Wait()
		close(t.resultChan)
		close(t.processChan)
	}()

	RetryTime := 0
	var bar *pb.ProgressBar
	if t.Config.UseProgressBar {
		bar = pb.Full.Start(len(t.Tasks))
	}
	// 获取结果
	for res := range t.resultChan {
		// 判断是否需要重试
		if res.Err != nil && res.RetryTime < t.Config.RetryMaxTime {
			// fmt.Println(res.Err)
			res.RetryTime++
			RetryTime++
			t.asyncRunTask(runTaskFunc, res)
		} else {
			if t.Config.UseProgressBar {
				bar.Increment()
			}
		}
	}
	if t.Config.UseProgressBar {
		bar.Finish()
	}
	return nil
}

func (t *Tasker) SyncRun(runTaskFunc TaskFunc) error {

	var bar *pb.ProgressBar
	if t.Config.UseProgressBar {
		bar = pb.Full.Start(len(t.Tasks))
	}
	for _, task := range t.Tasks {
		err := runTaskFunc(task)
		fmt.Println(err)
		if t.Config.UseProgressBar {
			bar.Increment()
		}
	}

	if t.Config.UseProgressBar {
		bar.Finish()
	}
	return nil
}

func (t *Tasker) Exec(isSync bool) error {
	var err error
	err = t.Build()
	if err != nil {
		return err
	}
	err = t.BuildTasks()
	if err != nil {
		return err
	}
	err = t.BeforeRun()
	if err != nil {
		return err
	}
	if isSync {
		err = t.SyncRun(t.RunTask)
	} else {
		err = t.Run(t.RunTask)
	}
	if err != nil {
		return err
	}
	return t.AfterRun()
}
