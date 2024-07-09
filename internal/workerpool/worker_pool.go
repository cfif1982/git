package workerpool

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

const countTasks = 10
const countWorkers = 3
const totalWorkTimeOut = time.Duration(10) * time.Second // таймаут общего времени работы всех воркеров
const taskTimeOut = time.Duration(4) * time.Second       // таймаут выполнения одной задачи

type Task struct {
	ID       int
	Duration time.Duration // время выполнения одной задачи
}

// выполнение задачи
func (t *Task) Do(ctx context.Context) {
	deadline, _ := ctx.Deadline() // узнаем время для отмены задачи, чтобы вывести эти данные
	sleepTime := time.Until(deadline)

	fmt.Printf("задача %d начата, продолжительность: %f, deadline: %f\n", t.ID, t.Duration.Seconds(), sleepTime.Seconds())

	for {
		select {
		// следим за контекстом для отмены работы
		case <-ctx.Done():
			fmt.Printf("задача %d отменена\n", t.ID)
			return
		// выводим сообщение о завершении работы
		case <-time.After(t.Duration):
			fmt.Printf("задача %d закончена\n", t.ID)
			return
		}
	}

}

type Worker struct {
	ID          int
	Queue       chan *Task // канал с очередью задач
	closeWorker bool       // флаг закрытия воркера
}

// начало работы воркера
func (w Worker) start(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()

	for {
		// если воркер помечен на закрытие, то завершаем задачу, а новую не берем и закрываемся
		if w.closeWorker {
			return
		}

		select {
		// берем задачу из очереди
		case t, ok := <-w.Queue:
			// если канал закрыт
			if !ok {
				fmt.Printf("worker %d закрыт \n", w.ID)
				return
			}

			// задаем контекст для отмены задачи
			ctxTaskTimeOut, taskCancelTimeOut := context.WithTimeout(ctx, taskTimeOut)
			defer taskCancelTimeOut()

			fmt.Printf("worker %d начал задачу %d \n", w.ID, t.ID)

			// выполняем задачу
			t.Do(ctxTaskTimeOut)

			// проверяем, как завершился контекст
			switch ctxTaskTimeOut.Err() {
			case context.Canceled:
				fmt.Printf("Прервали работу задачи %d\n", t.ID)
			case context.DeadlineExceeded:
				fmt.Printf("Истекло время работы задачи %d\n", t.ID)
			default:
				fmt.Printf("worker %d закончил задачу %d\n", w.ID, t.ID)
			}

		// следим за контекстом для отмены
		case <-ctx.Done():
			fmt.Printf("worker %d отменен\n", w.ID)
			return
		}

	}
}

// помечаем  воркера на закрытие
func (w Worker) close() {
	w.closeWorker = true
}

// генерируем задачи в очередь
func generator(queue chan<- *Task, countT int, wg *sync.WaitGroup) {
	defer wg.Done()

	// создаем задачи и помещаем их в очередь
	for i := 1; i <= countT; i++ {
		t := &Task{
			ID:       i,
			Duration: time.Duration((rand.Intn(9) + 1)) * time.Second, // случайное время на выполнение задачи
		}

		queue <- t

	}

	// закрывает канал тот , кто в него пишет
	close(queue)
}

func MainWorkerpool() {

	var wg sync.WaitGroup

	queueCH := make(chan *Task) // канал для передачи задач

	// Обработка сигналов завершения работы
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// создаем контекст для отены общего выполнения всех задач
	ctxTotalTimeOut, totalCancelTimeOut := context.WithTimeout(context.Background(), totalWorkTimeOut)
	defer totalCancelTimeOut()

	// запускаем таймер для отображения счетчика времени
	go timer()

	// созадем воркеров
	// мы можем здесь вернуть указатели на воркеров и хранить их в слайсе.
	// И в нужный момент обращаться к ним и закрывать, вызвав метод close()
	// функцию addWorker мы можем вызывать в любой момент и добавлять новых воркеров
	for i := 1; i <= countWorkers; i++ {
		addWorker(ctxTotalTimeOut, &wg, i, queueCH)
	}

	wg.Add(1)

	// генерируем задачи
	go generator(queueCH, countTasks, &wg)

	// Дополнительный канал для уведомления о завершении всех worker'ов
	doneChan := make(chan struct{})

	go func() {
		wg.Wait()
		close(doneChan)
	}()

	select {
	// следим за сигнальным каналом
	case <-sigChan:
		fmt.Println("Grasefull Shutdown start")
		totalCancelTimeOut()
	// следим за каналом окончания работы
	case <-doneChan:
		fmt.Println("все воркеры завершили работу")
	// следим за контекстом отмены всех работ
	case <-ctxTotalTimeOut.Done():
		fmt.Printf("время работы истекло\n")
	}

	// проверяем, как завершился контекст
	switch ctxTotalTimeOut.Err() {
	case context.Canceled:
		fmt.Println("Прервали общую работу")
	case context.DeadlineExceeded:
		fmt.Println("Истекло время общей работы")
	}

	fmt.Println("ФИНИШ")
}

func addWorker(ctx context.Context, wg *sync.WaitGroup, id int, queueCH chan *Task) {
	// созадем воркера
	worker := Worker{
		ID:          id,
		Queue:       queueCH,
		closeWorker: false,
	}
	wg.Add(1)

	// запускаем воркера в работу
	go worker.start(ctx, wg)
}

// таймер для отсчета времени
func timer() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	sec := 1
	for {
		select {
		case <-ticker.C:
			fmt.Printf("%d second\n", sec)
			sec++
		}
	}
}
