package main

import (
	"database/sql"
	"fmt"
	"os"
	"time"
)

// Инициализация базы данных
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

	// Проверяем, что база данных доступна
	if err := db.Ping(); err != nil {
		return fmt.Errorf("ошибка подключения к базе данных: %v", err)
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

// Обновление задачи в базе данных
func updateTaskInDB(task Task) error {

	// Рассчитываем следующую дату выполнения с использованием NextDate
	nextTaskDate, err := NextDate(time.Now(), task.Date, task.Repeat)
	if err != nil {
		return fmt.Errorf("не удалось вычислить следующую дату выполнения: %v", err)
	}

	// Обновляем задачу в базе данных
	query := `
		UPDATE scheduler
		SET date = ?, title = ?, comment = ?, repeat = ?
		WHERE id = ?;
	`
	result, err := db.Exec(query, nextTaskDate, task.Title, task.Comment, task.Repeat, task.ID)
	if err != nil {
		return fmt.Errorf("ошибка обновления задачи: %v", err)
	}

	// Проверяем, было ли обновлено хотя бы одно значение
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("ошибка проверки обновления: %v", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("задача с id=%d не найдена", task.ID)
	}

	return nil
}

// Сохранение новой задачи в базе данных
func saveTaskToDB(task Task) (int64, error) {
	const layout = "20060102"
	var err error
	var nextTaskDate string

	// Если повторение не указано
	if task.Repeat == "" {
		// Парсим дату задачи, если она указана
		var taskDate time.Time
		if task.Date != "" {
			taskDate, err = time.Parse(layout, task.Date)
			if err != nil {
				return -1, fmt.Errorf("ошибка при разборе даты: %v", err)
			}
		} else {
			// Если дата не указана, используем текущую
			taskDate = time.Now()
		}

		// Если дата задачи раньше текущей, подставляем текущую дату
		if taskDate.Before(time.Now()) {
			taskDate = time.Now()
		}

		// Форматируем дату в нужный формат
		nextTaskDate = taskDate.Format(layout)
	} else {
		// Если повторение указано, рассчитываем следующую дату с использованием функции NextDate
		nextTaskDate, err = NextDate(time.Now(), task.Date, task.Repeat)
		if err != nil {
			return 0, fmt.Errorf("не удалось вычислить следующую дату выполнения: %v", err)
		}
	}

	// Сохраняем задачу в базу данных
	query := `
		INSERT INTO scheduler (date, title, comment, repeat) 
		VALUES (?, ?, ?, ?);
	`
	result, err := db.Exec(query, nextTaskDate, task.Title, task.Comment, task.Repeat)
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
