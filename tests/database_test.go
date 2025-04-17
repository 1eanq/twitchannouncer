package tests

import (
	"database/sql"
	_ "database/sql"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"testing"
	"twitchannouncer/internal/database"
)

func TestStoreData(t *testing.T) {
	// Мокируем базу данных
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Ошибка при создании мок-базы: %v", err)
	}
	defer db.Close()

	// Создаем экземпляр DB
	dbInstance := &database.DB{DB: db}

	// Тестовые данные
	var data = database.Data{
		TwitchUsername:   "test_twitch",
		ChannelID:        -10012345,
		TelegramUsername: "test_telegram",
	} // Ожидаем, что запрос на проверку существования записи будет выполнен
	mock.ExpectQuery(`SELECT channel_id FROM users WHERE twitch_username = \? AND channel_id = \?`).
		WithArgs(data.TwitchUsername, data.ChannelID).
		WillReturnError(sql.ErrNoRows)

	// Ожидаем выполнение запроса на добавление данных
	mock.ExpectExec(`INSERT INTO users \(telegram_username, channel_id, twitch_username\)`).
		WithArgs(data.TelegramUsername, data.ChannelID, data.TwitchUsername).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Запускаем функцию StoreData
	err = dbInstance.StoreData(data)
	assert.NoError(t, err, "Ошибка при сохранении данных")

	// Проверяем, что все ожидаемые запросы были выполнены
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Не все ожидаемые запросы были выполнены: %v", err)
	}
}

func TestStoreData_Duplicate(t *testing.T) {
	// Мокируем базу данных
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Ошибка при создании мок-базы: %v", err)
	}
	defer db.Close()

	// Создаем экземпляр DB
	dbInstance := &database.DB{DB: db}

	// Тестовые данные
	data := database.Data{
		TwitchUsername:   "test_twitch",
		ChannelID:        -10012345,
		TelegramUsername: "test_telegram",
	}

	// Ожидаем, что запрос на проверку существования записи будет выполнен и возвращен результат
	mock.ExpectQuery(`SELECT channel_id FROM users WHERE twitch_username = \? AND channel_id = \?`).
		WithArgs(data.TwitchUsername, data.ChannelID).
		WillReturnRows(sqlmock.NewRows([]string{"channel_id"}).AddRow(data.ChannelID))

	// Запускаем функцию StoreData и проверяем на ошибку дублирования
	err = dbInstance.StoreData(data)
	assert.Error(t, err)
	assert.Equal(t, err.Error(), "Пользователь с таким twitch_username и channel_id уже существует", "Ошибка не совпала с ожидаемой")

	// Проверяем, что все ожидаемые запросы были выполнены
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Не все ожидаемые запросы были выполнены: %v", err)
	}
}

// Тест для функции DeleteData
func TestDeleteData(t *testing.T) {
	// Создаем мок базы данных
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Не удалось создать мок базы данных: %v", err)
	}
	defer db.Close()

	// Создаем экземпляр DB
	dbInstance := &database.DB{db}

	// Подготовка данных для теста
	data := database.Data{
		TelegramUsername: "test_telegram",
		TwitchUsername:   "test_twitch",
		ChannelID:        12345,
	}

	// Ожидаем, что будет выполнен запрос на проверку существования
	mock.ExpectQuery(`
		SELECT COUNT\(\*\) FROM users
		WHERE telegram_username = \? AND twitch_username = \? AND channel_id = \?
	`).
		WithArgs(data.TelegramUsername, data.TwitchUsername, data.ChannelID).
		WillReturnRows(sqlmock.NewRows([]string{"COUNT(*)"}).AddRow(1)) // Данные существуют

	// Ожидаем, что будет выполнен запрос на удаление данных
	mock.ExpectExec(`
		DELETE FROM users
		WHERE telegram_username = \? AND twitch_username = \? AND channel_id = \?
	`).
		WithArgs(data.TelegramUsername, data.TwitchUsername, data.ChannelID).
		WillReturnResult(sqlmock.NewResult(1, 1)) // Успешное удаление

	// Вызов функции
	err = dbInstance.DeleteData(data)

	// Проверки
	assert.NoError(t, err)
}

// Тест для функции DeleteData, когда данных нет
func TestDeleteDataNotFound(t *testing.T) {
	// Создаем мок базы данных
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Не удалось создать мок базы данных: %v", err)
	}
	defer db.Close()

	// Создаем экземпляр DB
	dbInstance := &database.DB{DB: db}

	// Подготовка данных для теста
	data := database.Data{
		TelegramUsername: "test_telegram",
		TwitchUsername:   "test_twitch",
		ChannelID:        12345,
	}

	// Ожидаем, что будет выполнен запрос на проверку существования
	mock.ExpectQuery(`
		SELECT COUNT\(\*\) FROM users
		WHERE telegram_username = \? AND twitch_username = \? AND channel_id = \?
	`).
		WithArgs(data.TelegramUsername, data.TwitchUsername, data.ChannelID).
		WillReturnRows(sqlmock.NewRows([]string{"COUNT(*)"}).AddRow(0)) // Данных не существует

	// Вызов функции
	err = dbInstance.DeleteData(data)

	// Проверки
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Такой подписки не существует")
}

func TestIfExists(t *testing.T) {
	// Мокируем базу данных
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Ошибка при создании мок-базы: %v", err)
	}
	defer db.Close()

	// Создаем экземпляр DB
	dbInstance := &database.DB{DB: db}

	// Тестовые данные
	data := database.Data{
		TwitchUsername:   "test_twitch",
		ChannelID:        -10012345,
		TelegramUsername: "test_telegram",
	}

	// Ожидаем, что запрос на проверку существования записи будет выполнен и вернет результат
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM users WHERE telegram_username = \? AND twitch_username = \? AND channel_id = \?`).
		WithArgs(data.TelegramUsername, data.TwitchUsername, data.ChannelID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	// Запускаем функцию IfExists
	exists, err := dbInstance.IfExists(data)
	assert.NoError(t, err, "Ошибка при проверке существования записи")
	assert.True(t, exists, "Запись должна существовать")

	// Проверяем, что все ожидаемые запросы были выполнены
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Не все ожидаемые запросы были выполнены: %v", err)
	}
}
