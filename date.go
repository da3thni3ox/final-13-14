package main

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Функция для получения следующей даты на основе заданных правил
func NextDate(nowIn time.Time, date string, repeat string) (string, error) {
	const layout = "20060102"

	var startDate time.Time
	var err error

	now, _ := time.Parse(layout, nowIn.Format(layout))

	// Если дата не указана, подставляем текущую дату
	if date == "" {
		startDate = now
	} else {
		startDate, err = time.Parse(layout, date)
		if err != nil {
			return "", fmt.Errorf("invalid date format: %v", err)
		}
	}

	// Если правило повторения не поддерживается
	switch {
	case strings.HasPrefix(repeat, "d "):
		// Повторение по дням
		daysStr := strings.TrimPrefix(repeat, "d ")
		days, err := strconv.Atoi(daysStr)

		if err != nil || days < 1 || days > 400 {
			return "", errors.New("invalid day interval")
		}
		nextDate := startDate

		// Если дата меньше сегодняшней, увеличиваем её до нужного интервала
		if nextDate.Before(now) {
			for nextDate.Before(now) || nextDate.Equal(now) {
				nextDate = nextDate.AddDate(0, 0, days)
			}
		} else if nextDate.Equal(now) {
			// Если дата равна сегодняшней, то это уже дата для повторения
			nextDate = now
		} else {
			// Если дата больше сегодняшней, просто добавляем интервал
			nextDate = nextDate.AddDate(0, 0, days)
		}

		// Проверка на 29 февраля
		if nextDate.Month() == time.February && nextDate.Day() == 29 {
			// Если следующий год не високосный, переходим на 1 марта
			if !isLeapYear(nextDate.Year()) {
				nextDate = time.Date(nextDate.Year(), time.March, 1, 0, 0, 0, 0, nextDate.Location())
			}
		}

		return nextDate.Format(layout), nil

	case repeat == "y":
		// Повторение по году
		nextDate := startDate

		// Год не меньше года текущего
		if nextDate.Year() < now.Year() {
			yearsDiff := now.Year() - nextDate.Year()
			fmt.Println(yearsDiff)
			nextDate = nextDate.AddDate(yearsDiff, 0, 0)
			return nextDate.Format(layout), nil
		}

		// Если дата начала меньше или равна текущей, увеличиваем на 1 год
		if !nextDate.After(now) {
			nextDate = nextDate.AddDate(1, 0, 0) // Увеличиваем на 1 год
		} else {
			nextDate = nextDate.AddDate(1, 0, 0) // Увеличиваем на 1 год
		}
		// Проверка на 29 февраля
		if nextDate.Month() == time.February && nextDate.Day() == 29 {
			// Если следующий год не високосный, переходим на 1 марта
			if !isLeapYear(nextDate.Year()) {
				nextDate = time.Date(nextDate.Year(), time.March, 1, 0, 0, 0, 0, nextDate.Location())
			}
		}

		return nextDate.Format(layout), nil

	case strings.HasPrefix(repeat, "m "):
		// Повторение по месяцам (например, "m 13")
		monthsStr := strings.TrimPrefix(repeat, "m ")
		monthsArr := strings.Split(monthsStr, ",")
		var months []int
		for _, m := range monthsArr {
			month, err := strconv.Atoi(m)
			if err != nil || month < 1 || month > 12 {
				return "", errors.New("invalid month value")
			}
			months = append(months, month)
		}

		nextDate := startDate
		for {
			if contains(months, int(nextDate.Month())) && nextDate.Before(now) {
				// Следующий месяц
				nextDate = nextDate.AddDate(0, 1, 0)
			} else if nextDate.After(now) {
				break
			}
		}

		return nextDate.Format(layout), nil

	case strings.HasPrefix(repeat, "w "):
		// Повторение по неделям (например, "w 1,2,3")
		weeksStr := strings.TrimPrefix(repeat, "w ")
		weeksArr := strings.Split(weeksStr, ",")
		var weeks []int
		for _, w := range weeksArr {
			week, err := strconv.Atoi(w)
			if err != nil || week < 1 || week > 7 {
				return "", errors.New("invalid week value")
			}
			weeks = append(weeks, week)
		}

		nextDate := startDate
		for {
			if contains(weeks, int(nextDate.Weekday())) && nextDate.Before(now) {
				// Следующая неделя
				nextDate = nextDate.AddDate(0, 0, 7)
			} else if nextDate.After(now) {
				break
			}
		}

		return nextDate.Format(layout), nil

	default:
		return "", fmt.Errorf("правило повторения указано в неправильном формате - %s", repeat)
	}
}

// Проверка на високосный год
func isLeapYear(year int) bool {
	if year%4 == 0 {
		if year%100 == 0 {
			if year%400 == 0 {
				return true
			}
			return false
		}
		return true
	}
	return false
}

// Вспомогательная функция для проверки присутствия значения в срезе
func contains(arr []int, val int) bool {
	for _, a := range arr {
		if a == val {
			return true
		}
	}
	return false
}
