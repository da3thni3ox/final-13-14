package main

import (
	"fmt"
	"strings"
	"time"
)

// nextDate вычисляет следующую дату для задачи в соответствии с правилом
func nextDate(currentDate time.Time, rule string) (time.Time, error) {
	parts := strings.Fields(rule)
	if len(parts) == 0 {
		return time.Time{}, fmt.Errorf("правило не указано")
	}

	switch parts[0] {
	case "d": // Добавить дни
		if len(parts) < 2 {
			return time.Time{}, fmt.Errorf("отсутствует количество дней в правиле: %s", rule)
		}
		var days int
		if _, err := fmt.Sscanf(parts[1], "%d", &days); err != nil || days < 1 || days > 400 {
			return time.Time{}, fmt.Errorf("неправильное правило: %s", rule)
		}
		return currentDate.AddDate(0, 0, days), nil

	case "y": // Добавить год
		return currentDate.AddDate(1, 0, 0), nil

	case "w": // Ближайший день недели
		if len(parts) < 2 {
			return time.Time{}, fmt.Errorf("отсутствуют дни недели в правиле: %s", rule)
		}
		days := parseInts(parts[1], 1, 7)
		if len(days) == 0 {
			return time.Time{}, fmt.Errorf("неправильные дни недели: %s", parts[1])
		}
		currentDay := int(currentDate.Weekday())
		if currentDay == 0 {
			currentDay = 7 // Преобразуем воскресенье в 7
		}
		for diff := 1; diff <= 7; diff++ {
			for _, day := range days {
				if (currentDay+diff-1)%7+1 == day {
					return currentDate.AddDate(0, 0, diff), nil
				}
			}
		}

	case "m": // Дни и месяцы
		if len(parts) < 2 {
			return time.Time{}, fmt.Errorf("отсутствуют дни или месяцы в правиле: %s", rule)
		}
		days, months := parseMonthlyRule(parts[1:])
		if len(days) == 0 {
			return time.Time{}, fmt.Errorf("неправильное правило: %s", rule)
		}
		for {
			if len(months) == 0 || contains(months, int(currentDate.Month())) {
				for _, day := range days {
					candidate := resolveDay(currentDate.Year(), int(currentDate.Month()), day)
					if candidate.After(currentDate) {
						return candidate, nil
					}
				}
			}
			currentDate = currentDate.AddDate(0, 1, 0) // Следующий месяц
		}

	default:
		return time.Time{}, fmt.Errorf("неизвестное правило: %s", rule)
	}
	return time.Time{}, nil
}

// parseInts парсит список чисел из строки
func parseInts(input string, min, max int) []int {
	parts := strings.Split(input, ",")
	var result []int
	for _, part := range parts {
		var value int
		if _, err := fmt.Sscanf(part, "%d", &value); err == nil && value >= min && value <= max {
			result = append(result, value)
		}
	}
	return result
}

// parseMonthlyRule парсит дни и месяцы из правила
func parseMonthlyRule(parts []string) ([]int, []int) {
	days := parseInts(parts[0], -31, 31)
	months := []int{}
	if len(parts) > 1 {
		months = parseInts(parts[1], 1, 12)
	}
	return days, months
}

// resolveDay возвращает корректную дату для указанного дня месяца
func resolveDay(year, month, day int) time.Time {
	if day > 0 {
		return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	}
	lastDay := time.Date(year, time.Month(month+1), 0, 0, 0, 0, 0, time.UTC).Day()
	return time.Date(year, time.Month(month), lastDay+day+1, 0, 0, 0, 0, time.UTC)
}

// contains проверяет, содержится ли элемент в списке
func contains(list []int, elem int) bool {
	for _, v := range list {
		if v == elem {
			return true
		}
	}
	return false
}
