package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"go-absen/domain/user"
	"go-absen/entities"
	"go-absen/helper/hashing"
	"gorm.io/gorm"
	"log"
	"net/http"
	"time"
)

type UserService struct {
	repo    user.UserRepositoryInterface
	hashing hashing.HashInterface
}

func NewUserService(repo user.UserRepositoryInterface, hashing hashing.HashInterface) user.UserServiceInterface {
	return &UserService{repo: repo, hashing: hashing}
}

func (s *UserService) GetId(id int) (*entities.UserEntity, error) {
	result, err := s.repo.FindId(id)
	if err != nil {
		return nil, errors.New("id not found")
	}
	return result, nil
}

func (s *UserService) GetEmail(email string) (*entities.UserEntity, error) {
	result, err := s.repo.FindEmail(email)
	if err != nil {
		return nil, errors.New("your email has been already")
	}
	return result, nil
}

func (s *UserService) GetNik(nik string) (*entities.UserEntity, error) {
	result, err := s.repo.FindNik(nik)
	if err != nil {
		return nil, errors.New("your nik has been already")
	}
	return result, nil
}

func (s *UserService) RecordAttendance(userID int, latitude, longitude float64) (*entities.AttendanceEntity, error) {
	// Mengecek apakah user sudah melakukan absensi pada hari yang sama
	currentDate := time.Now().Format("2006-01-02") // Format: YYYY-MM-DD
	existingAttendance, err := s.repo.GetAttendanceByDate(userID, currentDate)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		// Mengembalikan error jika terjadi kesalahan selain data tidak ditemukan
		return nil, errors.New("error checking existing attendance")
	}

	if existingAttendance != nil {
		// Jika user sudah melakukan absensi pada hari yang sama
		return nil, errors.New("user has already recorded attendance for today")
	}

	// Jika user belum melakukan absensi pada hari yang sama, lanjutkan proses absensi
	location, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		// Handle error jika gagal memuat zona waktu
		return nil, err
	}

	attendance := &entities.AttendanceEntity{
		UserID:    userID,
		Latitude:  latitude,
		Longitude: longitude,
		CreatedAt: time.Now().In(location).Unix(), // Menggunakan Unix epoch time di zona waktu "Asia/Jakarta"
	}

	// Menetapkan status absensi berdasarkan waktu absensi (lokal)
	status, err := calculateAttendanceStatus(attendance.CreatedAt)
	if err != nil {
		return nil, err
	}

	if status == "Early" {
		return nil, errors.New("Anda belum bisa absen pada hari ini, absen dimulai jam 7 pagi sampai jam 8 pagi")
	}

	attendance.Status = status

	createdAttendance, err := s.repo.InsertAttendance(attendance)
	if err != nil {
		return nil, errors.New("error recording attendance")
	}

	return createdAttendance, nil
}

func (s *UserService) GetLocationName(latitude, longitude float64) (string, error) {
	url := fmt.Sprintf("https://nominatim.openstreetmap.org/reverse?format=json&lat=%f&lon=%f", latitude, longitude)
	resp, err := http.Get(url)
	if err != nil {
		log.Println("Error calling Nominatim API:", err)
		return "", err
	}
	defer resp.Body.Close()

	// Unmarshal body JSON directly without reading the response body separately
	var geocodingResponse map[string]interface{}
	if err := s.UnmarshalResponseBody(resp, &geocodingResponse); err != nil {
		log.Println("Error unmarshalling Nominatim API response:", err)
		return "", err
	}

	// Ambil display_name dari respons JSON
	displayName, ok := geocodingResponse["display_name"].(string)
	if !ok {
		log.Println("Invalid Nominatim API response format")
		return "", errors.New("invalid response format")
	}

	return displayName, nil
}
func (s *UserService) UnmarshalResponseBody(response *http.Response, v interface{}) error {
	// Use json.NewDecoder directly on response.Body
	if err := json.NewDecoder(response.Body).Decode(&v); err != nil {
		return err
	}

	return nil
}
func (s *UserService) GetAttendanceHistory(userID int) ([]entities.AttendanceEntity, error) {
	attendances, err := s.repo.GetAttendanceHistory(userID)
	if err != nil {
		return nil, errors.New("error getting attendance history")
	}

	return attendances, nil
}

func calculateAttendanceStatus(createdAt int64) (string, error) {
	const (
		onTimeHour      = 7
		onTimeEndHour   = 8
		warningHour     = 8
		warningMinute   = 30
		absentStartHour = 7
		absentEndHour   = 8
	)

	// Ubah waktu Unix epoch ke dalam objek waktu UTC
	attendanceTime := time.Unix(createdAt, 0).UTC()

	// Ubah waktu UTC ke dalam objek waktu "Asia/Jakarta"
	location, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		return "", err
	}
	attendanceTime = attendanceTime.In(location)

	if attendanceTime.Hour() < onTimeHour || (attendanceTime.Hour() == onTimeHour && attendanceTime.Minute() < warningMinute) {
		return "On-Time", nil
	} else if attendanceTime.Hour() >= onTimeHour && attendanceTime.Hour() < onTimeEndHour {
		return "Warning", nil
	} else if attendanceTime.Hour() >= onTimeEndHour && attendanceTime.Hour() < absentEndHour {
		return "Late", nil
	} else if attendanceTime.Hour() < absentStartHour {
		return "Early", nil
	} else {
		return "", errors.New("Invalid attendance time")
	}
}
