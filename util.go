package main

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/jinzhu/gorm"
	"github.com/spf13/viper"
)

//Error types:
//
//Invalid Session Token
//Expired Session Token
//Unregistered User
//Invalid Password
//First Login Change Password
//Authentication Failed
//Encryption Failed
//Database Error
//Permission Denied

var (
	TRACE    *log.Logger
	INFO     *log.Logger
	WARNING  *log.Logger
	ERROR    *log.Logger
	DATABASE *log.Logger

	InvalidSessionToken      = errors.New("Invalid Session Token")
	ExpiredSessionToken      = errors.New("Expired Session Token")
	UnregisteredUser         = errors.New("Unregistered User")
	IncorrectPassword        = errors.New("Invalid Password")
	FirstLoginPasswordChange = errors.New("First Login Change Password")
	AuthenticationFailed     = errors.New("Authentication Failed")
	EncryptionError          = errors.New("Encryption Failed")
	DatabaseError            = errors.New("Database Error")
	PermissionDenied         = errors.New("Permission Denied")
	SystemError              = errors.New("System Error")
	DuplicateUsernameError   = errors.New("Duplicate username")
)

//Constant values:
//For web use 1
//For mobile use 2
const (
	web    = 1
	mobile = 2

	LOGIN           = "LOGIN"
	PASSWORD_CHANGE = "PASSWORD_CHANGE"
)

//Initializes the app by setting up logging file, defaults to stdout in case of error opening the specified file.
//Opens the DB connection
//Initialized DB with tables if -initdb=true option is passed when starting the app
func InitApp(logFileName string, initializeDB *bool) {
	var err error
	file, err = os.OpenFile(logFileName, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0666)
	if err != nil {
		fmt.Println("Log file cannot be opened")
		file.Close()
		file = os.Stdout
	}

	InitLogger(file, file, file, file, file)
	TRACE.Println("Logger initialized to file " + logFileName)

	dbConnection = GetDBConnection()
	TRACE.Println("Global connection object initialized")

	TRACE.Println(string(strconv.AppendBool([]byte("Application started with Init DB flag value "), *initializeDB)))

	if *initializeDB == true {
		TRACE.Println("DB Init Start")
		InitDB()
		TRACE.Println("DB Init Complete")
	}

	LoadAppPropertiesFromDb()

	dbLoggerPropertyValue, err := GetAppProperties("DbDebugLogs")

	if err == nil {
		dbLoggerFlag, err := strconv.ParseBool(dbLoggerPropertyValue)
		if err == nil && dbLoggerFlag {
			dbConnection.LogMode(dbLoggerFlag)
			dbConnection.SetLogger(DATABASE)
			TRACE.Println("Database logger initialized")
		}
	}

	StartScheduledJobs()
}

func InitLogger(traceHandle io.Writer, infoHandle io.Writer, warningHandle io.Writer, errorHandle io.Writer, databaseHandle io.Writer) {

	TRACE = log.New(traceHandle, "TRACE: ", log.Ldate|log.Ltime|log.Lshortfile)

	INFO = log.New(infoHandle, "INFO: ", log.Ldate|log.Ltime|log.Lshortfile)

	WARNING = log.New(warningHandle, "WARNING: ", log.Ldate|log.Ltime|log.Lshortfile)

	ERROR = log.New(errorHandle, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile)

	DATABASE = log.New(databaseHandle, "DATABASE: ", log.Ldate|log.Ltime|log.Lshortfile)
}

func GetAppProperties(propertyName string) (string, error) {
	for index := range properties {
		if properties[index].PropertyName == propertyName {
			return properties[index].PropertyValue, nil
		}
	}
	return "", errors.New("App property " + propertyName + " not set")
}

func LoadConfigurationFromFile() {
	TRACE.Println("Loading configuration from " + configName)

	viper.SetConfigName(configName)
	viper.AddConfigPath(".")
	err := viper.ReadInConfig()

	if err != nil {
		panic("Configuration couldn't be initialized, panicking now " + err.Error())
	}
}

func LoadAppPropertiesFromDb() {
	dbConnection.Find(&properties)
	TRACE.Println("App properties array initialized at: " + time.Now().String())
}

func ArchiveTokenAfterCutOffTime(db *gorm.DB) {

	tokenCutOffTimeString, err := GetAppProperties("TokenCutOffTime")
	if err != nil {
		ERROR.Println(err.Error())
	}

	tokenCutOffTimeInteger, err := strconv.Atoi(tokenCutOffTimeString)
	if err != nil {
		ERROR.Println("String to integer conversion failed for TokenCutOffTime, reverting to default value = 30 minutes")
		tokenCutOffTimeInteger = 30
	}

	TRACE.Println("Token archiving started at: " + time.Now().String())

	db.Exec("UPDATE TABLE tokens SET active = ? WHERE (SELECT EXTRACT(MINUTE FROM (? - expires_at)) FROM tokens) > ?", false, time.Now().UTC(), tokenCutOffTimeInteger)

	db.Exec("INSERT INTO archived_tokens (token, user_id, key, created_at, last_accessed_at, expires_at, device_type_id, active)(SELECT (token, user_id, key, created_at, last_accessed_at, expires_at, device_type_id, active) WHERE active = ?)", false)

	db.Exec("DELETE FROM tokens WHERE active = ?", false)

	TRACE.Println("Token archiving completed at: " + time.Now().String())
}
