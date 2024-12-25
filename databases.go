package main

import (
	"database/sql"
	"fmt"
	"os"
	"time"
)

func initDb(config config) error {
	var migration bool

	// Проверяем, существует ли файл базы данных
	if _, err := os.Stat(config.DbFilePath); os.IsNotExist(err) {
		migration = true
	}

	// Открываем базу данных
	var err error
	db, err = sql.Open("sqlite", config.DbFilePath)
	if err != nil {
		return fmt.Errorf("ошибка открытия базы данных: %v", err)
	}

	if migration {
		// Создаём таблицу, если её нет
		_, err = db.Exec(`
			CREATE TABLE IF NOT EXISTS scheduler (
				id INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL,
				date TEXT NOT NULL,
				title TEXT NOT NULL,
				comment TEXT,
				repeat TEXT CHECK (LENGTH(repeat) <= 128)
			);
		`)
		if err != nil {
			return fmt.Errorf("ошибка создания таблицы: %v", err)
		}

		// Создаём индекс для ускорения запросов
		_, err = db.Exec(`
			CREATE INDEX IF NOT EXISTS scheduler_date ON scheduler(date);
		`)
		if err != nil {
			return fmt.Errorf("ошибка создания индекса: %v", err)
		}
	}

	return nil
}

func updateTaskInDB(task Task) error {
	// Парсим дату задачи
	var taskDate time.Time
	var err error

	if task.Date != "" {
		// Если дата указана, пытаемся её распарсить
		taskDate, err = time.Parse("20060102", task.Date)
		if err != nil {
			return fmt.Errorf("неправильный формат даты, ожидается YYYYMMDD: %v", err)
		}
	} else {
		// Если дата не указана, используем текущую
		taskDate = time.Now()
	}

	// Проверяем правило повторения
	if task.Repeat != "" {
		_, err := nextDate(taskDate, task.Repeat)
		if err != nil {
			return fmt.Errorf("не удалось вычислить следующую дату выполнения: %v", err)
		}
	}

	// Обновляем задачу в базе данных
	query := `
		UPDATE scheduler
		SET date = ?, title = ?, comment = ?, repeat = ?
		WHERE id = ?;
	`
	result, err := db.Exec(query, taskDate.Format("20060102"), task.Title, task.Comment, task.Repeat, task.ID)
	if err != nil {
		return fmt.Errorf("ошибка обновления задачи: %v", err)
	}

	// Проверяем, было ли обновлено хотя бы одно значение
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("ошибка проверки обновления: %v", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("задача с id=%s не найдена", task.ID)
	}

	return nil
}

func saveTaskToDB(task Task) (int64, error) {
	// Парсим дату задачи
	var taskDate time.Time
	var err error

	// Если дата не указана, подставляем сегодняшнюю
	if task.Date == "" {
		taskDate = time.Now()
	} else {
		// Если дата указана, пытаемся её распарсить
		taskDate, err = time.Parse("20060102", task.Date)
		if err != nil {
			return -1, fmt.Errorf("неправильный формат даты, ожидается YYYYMMDD: %v", err)
		}
	}

	// Если дата меньше сегодняшнего дня, подставляем текущую дату
	if taskDate.Before(time.Now()) {
		taskDate = time.Now()
	}

	// Если правило повторения пустое или отсутствует, дата остаётся сегодняшней
	if task.Repeat == "" {
		// Ничего не делаем, сохраняем текущую дату
	} else {
		// Проверяем правило повторения для валидации
		_, err := nextDate(taskDate, task.Repeat)
		if err != nil {
			return -1, fmt.Errorf("не удалось вычислить следующую дату выполнения: %v", err)
		}
	}

	// Сохраняем задачу в базу данных
	query := `
	INSERT INTO scheduler (date, title, comment, repeat) 
	VALUES (?, ?, ?, ?);
	`
	result, err := db.Exec(query, taskDate.Format("20060102"), task.Title, task.Comment, task.Repeat)
	if err != nil {
		return 0, fmt.Errorf("ошибка сохранения задачи: %v", err)
	}

	// Получаем ID последней вставленной записи
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("ошибка получения ID задачи: %v", err)
	}

	return id, nil
}
