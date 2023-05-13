// Package godler  provides ...
package tasker

import (
	"sync"

	"github.com/cheggaaa/pb/v3"
)

type TaskFunc func(*Task) error

type ITasker interface {
	Build() error
	BuildTasks() error
	AddTask(*Task)
	GetTasks() []*Task
	RunTask(*Task) error
	Run(TaskFunc) error
	SyncRun(TaskFunc) error
	AfterRun() error
	BeforeRun() error
	Progress() float64
}

// 执行任务
func ExecTasker(t ITasker, isSync bool) error {
	// TODO: afterrun 执行顺序
	var err error
	if len(t.GetTasks()) == 0 {
		err = t.Build()
		if err != nil {
			return err
		}
		err = t.BuildTasks()
		if err != nil {
			return err
		}
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

// 获取任务进度
// 建议 NewTask 方法精确的传入 done 参数
func GetTaskerProgress(t ITasker) (p float64, err error) {
	if len(t.GetTasks()) == 0 {
		err = t.Build()
		if err != nil {
			return
		}
		err = t.BuildTasks()
		if err != nil {
			return
		}
	}
	p = t.Progress()
	return
}

// 新建任务体
// info: 具体的任务
// done: 该任务是否已经完成，正确的传入此值可以提升效率
func NewTask(info interface{}, done bool) *Task {
	return &Task{Info: info, IsDone: done}
}

type Task struct {
	RetryTime int         `json:"retry_count"`
	Err       error       `json:"err"`
	IsDone    bool        `json:"done"`
	Info      interface{} `json:"info"`
}

func (t *Task) Done() *Task {
	t.IsDone = true
	return t
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
		tasks:       make([]*Task, 0),
	}
}

type Tasker struct {
	TaskId string
	Config *TaskerConfig

	tasks       []*Task
	processChan chan bool
	resultChan  chan *Task
	waitGroup   sync.WaitGroup
	pbar        *pb.ProgressBar
}

func (t *Tasker) BuildTasks() error { return nil }

func (t *Tasker) Build() error { return nil }

func (t *Tasker) AfterRun() error { return nil }

func (t *Tasker) BeforeRun() error { return nil }

func (t *Tasker) RunTask(task *Task) error { return nil }

func (t *Tasker) AddTask(task *Task) {
	t.tasks = append(t.tasks, task)
}

func (t *Tasker) GetTasks() []*Task {
	return t.tasks
}

func (t *Tasker) GetErrorTasks() []*Task {
	tasks := make([]*Task, 0)
	for _, t := range t.GetTasks() {
		if t.Err != nil {
			tasks = append(tasks, t)
		}
	}
	return tasks
}

func (t *Tasker) asyncRunTask(runTaskFunc TaskFunc, task *Task) {
	t.waitGroup.Add(1)
	go func(task *Task) {
		defer t.waitGroup.Done()
		t.processChan <- true
		if !task.IsDone {
			err := runTaskFunc(task)
			if err != nil {
				task.Err = err
			} else {
				task.Done()
			}
		}
		<-t.processChan
		t.resultChan <- task
	}(task)
}

// 开始加载进度条
func (t *Tasker) pbStart() error {
	if t.Config.UseProgressBar {
		t.pbar = pb.Full.Start(len(t.GetTasks()))
	}
	return nil
}

// 增加进度
func (t *Tasker) pbIncr() {
	if t.Config.UseProgressBar {
		t.pbar.Increment()
	}
}

// 结束进度
func (t *Tasker) pbFinish() {
	if t.Config.UseProgressBar {
		t.pbar.Finish()
	}
}

func (t *Tasker) Run(runTaskFunc TaskFunc) error {

	for _, task := range t.GetTasks() {
		t.asyncRunTask(runTaskFunc, task)
	}

	go func() {
		t.waitGroup.Wait()
		close(t.resultChan)
		close(t.processChan)
	}()

	RetryTime := 0
	t.pbStart()
	// 获取结果
	for res := range t.resultChan {
		// 判断是否需要重试
		if res.Err != nil && res.RetryTime < t.Config.RetryMaxTime {
			// fmt.Println(res.Err)
			res.RetryTime++
			RetryTime++
			t.asyncRunTask(runTaskFunc, res)
		} else {
			t.pbIncr()
		}
	}
	t.pbFinish()
	return nil
}

func (t *Tasker) SyncRun(runTaskFunc TaskFunc) error {
	t.pbStart()
	for _, task := range t.GetTasks() {
		err := runTaskFunc(task)
		if err != nil {
			task.Err = err
		} else {
			t.pbIncr()
		}
	}
	t.pbFinish()
	return nil
}

// 获取进度
func (t *Tasker) Progress() float64 {
	var done float64
	for _, t := range t.GetTasks() {
		if t.IsDone {
			done++
		}
	}
	p := done / float64(len(t.GetTasks()))
	return p
}
