package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/spf13/viper"
)

// StudentInterface 定义学生接口
type StudentInterface interface {
	GetID() int
	GetName() string
	GetGender() string
	GetClass() string
	GetScores() map[string]float64
	SetScores(scores map[string]float64)
}

// Student 结构体
type Student struct {
	Name      string             `json:"name"`
	StudentID int                `json:"id"`
	Gender    string             `json:"gender"`
	Class     string             `json:"class"`
	Scores    map[string]float64 `json:"scores"`
}

// Undergraduate 本科生结构体
type Undergraduate struct {
	Student
}

// Graduate 研究生结构体
type Graduate struct {
	Student
}

// 实现 StudentInterface 接口方法

// GetID 获取学生ID
func (u *Undergraduate) GetID() int {
	return u.StudentID
}

// GetName 获取学生姓名
func (u *Undergraduate) GetName() string {
	return u.Name
}

// GetGender 获取学生性别
func (u *Undergraduate) GetGender() string {
	return u.Gender
}

// GetClass 获取学生班级
func (u *Undergraduate) GetClass() string {
	return u.Class
}

// GetScores 获取学生成绩
func (u *Undergraduate) GetScores() map[string]float64 {
	return u.Scores
}

// SetScores 设置学生成绩
func (u *Undergraduate) SetScores(scores map[string]float64) {
	u.Scores = scores
}

// GetID 获取学生ID
func (g *Graduate) GetID() int {
	return g.StudentID
}

// GetName 获取学生姓名
func (g *Graduate) GetName() string {
	return g.Name
}

// GetGender 获取学生性别
func (g *Graduate) GetGender() string {
	return g.Gender
}

// GetClass 获取学生班级
func (g *Graduate) GetClass() string {
	return g.Class
}

// GetScores 获取学生成绩
func (g *Graduate) GetScores() map[string]float64 {
	return g.Scores
}

// SetScores 设置学生成绩
func (g *Graduate) SetScores(scores map[string]float64) {
	g.Scores = scores
}

// Database 结构体，作为基于内存的 NoSQL 数据库
type Database struct {
	data map[string]interface{}
	mu   sync.RWMutex
	rdb  *redis.Client
}

// NewDatabase 初始化 Database
func NewDatabase(rdb *redis.Client) *Database {
	return &Database{
		data: make(map[string]interface{}),
		rdb:  rdb,
	}
}

// Insert 插入键值对
func (db *Database) Insert(key string, value interface{}) {
	db.mu.Lock()
	defer db.mu.Unlock()
	db.data[key] = value
	// 同时更新 Redis
	jsonData, err := json.Marshal(value)
	if err != nil {
		fmt.Printf("Error marshaling data for Redis: %v\n", err)
		return
	}
	err = db.rdb.Set(db.rdb.Context(), key, jsonData, 0).Err()
	if err != nil {
		fmt.Printf("Error setting data in Redis: %v\n", err)
	}
}

// Delete 根据键删除键值对
func (db *Database) Delete(key string) {
	db.mu.Lock()
	defer db.mu.Unlock()
	delete(db.data, key)
	// 同时从 Redis 删除
	err := db.rdb.Del(db.rdb.Context(), key).Err()
	if err != nil {
		fmt.Printf("Error deleting data from Redis: %v\n", err)
	}
}

// Update 更新键值对的值
func (db *Database) Update(key string, value interface{}) {
	db.mu.Lock()
	defer db.mu.Unlock()
	if _, exists := db.data[key]; exists {
		db.data[key] = value
		// 同时更新 Redis
		jsonData, err := json.Marshal(value)
		if err != nil {
			fmt.Printf("Error marshaling data for Redis: %v\n", err)
			return
		}
		err = db.rdb.Set(db.rdb.Context(), key, jsonData, 0).Err()
		if err != nil {
			fmt.Printf("Error setting data in Redis: %v\n", err)
		}
	}
}

// Query 根据键查询值
func (db *Database) Query(key string) (interface{}, bool) {
	db.mu.RLock()
	value, exists := db.data[key]
	db.mu.RUnlock()

	if !exists {
		// 懒加载：从 Redis 中获取数据
		db.mu.Lock()
		defer db.mu.Unlock()
		val, err := db.rdb.Get(db.rdb.Context(), key).Bytes()
		if err == nil {
			var data interface{}
			err = json.Unmarshal(val, &data)
			if err == nil {
				db.data[key] = data
				return data, true
			}
		}
		return nil, false
	}
	return value, true
}

// Count 获取当前存储的元素个数
func (db *Database) Count() int {
	db.mu.RLock()
	defer db.mu.RUnlock()
	return len(db.data)
}

// StudentManager 结构体
type StudentManager struct {
	students map[int]*Student
	mu       sync.Mutex
	db       *Database
}

// NewStudentManager 初始化 StudentManager
// 初始化了一个空的学生映射，用于后续添加和管理学生信息
func NewStudentManager(rdb *redis.Client) *StudentManager {
	db := NewDatabase(rdb)
	return &StudentManager{
		students: make(map[int]*Student),
		db:       db,
	}
}

// AddStudent 添加学生信息
func (sm *StudentManager) AddStudent(student StudentInterface) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	id := student.GetID()
	sm.students[id] = &Student{
		Name:      student.GetName(),
		StudentID: student.GetID(),
		Gender:    student.GetGender(),
		Class:     student.GetClass(),
		Scores:    student.GetScores(),
	}
	// 同时插入到内存数据库
	sm.db.Insert(strconv.Itoa(id), sm.students[id])
}

// DeleteStudent 删除学生信息
func (sm *StudentManager) DeleteStudent(studentID int) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if _, exists := sm.students[studentID]; exists {
		delete(sm.students, studentID)
		// 同时从内存数据库删除
		sm.db.Delete(strconv.Itoa(studentID))
		return nil
	}
	return fmt.Errorf("student with ID %d not found", studentID)
}

// ModifyStudent 修改学生信息
func (sm *StudentManager) ModifyStudent(studentID int, updates map[string]interface{}) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	student, exists := sm.students[studentID]
	if !exists {
		return fmt.Errorf("student with ID %d not found", studentID)
	}
	for key, value := range updates {
		switch key {
		case "name":
			if name, ok := value.(string); ok {
				student.Name = name
			}
		case "gender":
			if gender, ok := value.(string); ok {
				student.Gender = gender
			}
		case "class":
			if class, ok := value.(string); ok {
				student.Class = class
			}
		case "scores":
			if scores, ok := value.(map[string]float64); ok {
				student.Scores = scores
			}
		}
	}
	// 同时更新内存数据库
	sm.db.Update(strconv.Itoa(studentID), student)
	return nil
}

// AddScore 为学生添加成绩
func (sm *StudentManager) AddScore(studentID int, courseName string, score float64) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	student, exists := sm.students[studentID]
	if !exists {
		return fmt.Errorf("student with ID %d not found", studentID)
	}
	if student.Scores == nil {
		student.Scores = make(map[string]float64)
	}
	student.Scores[courseName] = score
	// 同时更新内存数据库
	sm.db.Update(strconv.Itoa(studentID), student)
	return nil
}

// DeleteScore 删除学生成绩
func (sm *StudentManager) DeleteScore(studentID int, courseName string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	student, exists := sm.students[studentID]
	if !exists {
		return fmt.Errorf("student with ID %d not found", studentID)
	}
	if student.Scores != nil {
		delete(student.Scores, courseName)
		// 同时更新内存数据库
		sm.db.Update(strconv.Itoa(studentID), student)
	}
	return nil
}

// ModifyScore 修改学生成绩
func (sm *StudentManager) ModifyScore(studentID int, courseName string, score float64) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	student, exists := sm.students[studentID]
	if !exists {
		return fmt.Errorf("student with ID %d not found", studentID)
	}
	if student.Scores != nil {
		student.Scores[courseName] = score
		// 同时更新内存数据库
		sm.db.Update(strconv.Itoa(studentID), student)
	}
	return nil
}

// QueryStudent 查询学生信息
func (sm *StudentManager) QueryStudent(studentID int) (*Student, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	student, exists := sm.students[studentID]
	if !exists {
		// 尝试从数据库查询
		value, found := sm.db.Query(strconv.Itoa(studentID))
		if found {
			if s, ok := value.(*Student); ok {
				sm.students[studentID] = s
				return s, nil
			}
		}
		return nil, fmt.Errorf("student with ID %d not found", studentID)
	}
	return student, nil
}

// QueryScore 查询学生成绩
func (sm *StudentManager) QueryScore(studentID int, courseName string) (float64, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	student, exists := sm.students[studentID]
	if !exists {
		// 尝试从数据库查询
		value, found := sm.db.Query(strconv.Itoa(studentID))
		if found {
			if s, ok := value.(*Student); ok {
				sm.students[studentID] = s
				student = s
			}
		}
	}
	if student == nil {
		return 0, fmt.Errorf("student with ID %d not found", studentID)
	}
	if student.Scores == nil {
		return 0, fmt.Errorf("student has no scores")
	}
	score, exists := student.Scores[courseName]
	if !exists {
		return 0, fmt.Errorf("score for course %s not found", courseName)
	}
	return score, nil
}

// Node 节点结构体
type Node struct {
	ID       string
	Address  string
	Database *Database
	Peers    []string
}

// NewNode 初始化节点
func NewNode(id, address string, db *Database, peers []string) *Node {
	return &Node{
		ID:       id,
		Address:  address,
		Database: db,
		Peers:    peers,
	}
}

// SyncWithPeer 与其他节点同步数据
func (n *Node) SyncWithPeer(peerAddress string) error {
	// 获取本地数据库数据
	n.Database.mu.RLock()
	data := make(map[string]interface{})
	for k, v := range n.Database.data {
		data[k] = v
	}
	n.Database.mu.RUnlock()

	// 将数据转换为 JSON
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	// 发送 POST 请求到其他节点
	resp, err := http.Post(peerAddress+"/gossip/sync", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// 读取响应
	_, err = io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	return nil
}

// HandleSync 处理其他节点的同步请求
func (n *Node) HandleSync(c *gin.Context) {
	var incomingData map[string]interface{}
	if err := c.ShouldBindJSON(&incomingData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 合并数据
	n.Database.mu.Lock()
	for k, v := range incomingData {
		n.Database.data[k] = v
	}
	n.Database.mu.Unlock()

	c.JSON(http.StatusOK, gin.H{"message": "Sync successful"})
}

// StartGossip 启动 Gossip 协议
func (n *Node) StartGossip() {
	rand.Seed(time.Now().UnixNano())
	ticker := time.NewTicker(5 * time.Second)
	for range ticker.C {
		if len(n.Peers) > 0 {
			// 随机选择一个节点
			peerIndex := rand.Intn(len(n.Peers))
			peerAddress := n.Peers[peerIndex]
			if err := n.SyncWithPeer(peerAddress); err != nil {
				fmt.Printf("Error syncing with peer %s: %v\n", peerAddress, err)
			}
		}
	}
}

func init() {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	if err := viper.ReadInConfig(); err != nil {
		panic(fmt.Errorf("Fatal error config file: %w", err))
	}
}

func preloadData(db *Database) {
	// 这里可以实现只加载一部分热点 key 的逻辑
	// 目前简单地将 Redis 中所有数据加载到内存
	keys, err := db.rdb.Keys(db.rdb.Context(), "*").Result()
	if err != nil {
		fmt.Printf("Error getting keys from Redis: %v\n", err)
		return
	}
	for _, key := range keys {
		val, err := db.rdb.Get(db.rdb.Context(), key).Bytes()
		if err == nil {
			var data interface{}
			err = json.Unmarshal(val, &data)
			if err == nil {
				db.mu.Lock()
				db.data[key] = data
				db.mu.Unlock()
			}
		}
	}
}

func main() {
	r := gin.Default()

	// 初始化 Redis 客户端
	redisAddr := viper.GetString("redis.address")
	redisPassword := viper.GetString("redis.password")
	redisDB := viper.GetInt("redis.db")
	rdb := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: redisPassword,
		DB:       redisDB,
	})

	studentManager := NewStudentManager(rdb)

	// 缓存预热
	preloadData(studentManager.db)

	// 读取环境变量获取节点信息
	nodeID := os.Getenv("NODE_ID")
	nodeAddress := os.Getenv("NODE_ADDRESS")
	peersStr := os.Getenv("PEERS")
	peers := strings.Split(peersStr, ",")

	node := NewNode(nodeID, nodeAddress, studentManager.db, peers)

	// 根据配置启动一致性算法
	consistencyAlgorithm := viper.GetString("consistency_algorithm")
	if consistencyAlgorithm == "gossip" {
		go node.StartGossip()
	} else if consistencyAlgorithm == "raft" {
		fmt.Println("Raft protocol is not implemented yet.")
	}

	// 添加 Gossip 同步接口
	r.POST("/gossip/sync", func(c *gin.Context) {
		node.HandleSync(c)
	})

	// 添加学生信息接口
	r.POST("/students", func(c *gin.Context) {
		var student Student
		if err := c.ShouldBindJSON(&student); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		undergraduate := &Undergraduate{Student: student}
		studentManager.AddStudent(undergraduate)
		c.JSON(http.StatusOK, gin.H{"message": "Student added successfully"})
	})

	// 删除学生信息接口
	r.DELETE("/students/:id", func(c *gin.Context) {
		studentID, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid student ID"})
			return
		}
		if err := studentManager.DeleteStudent(studentID); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "Student deleted successfully"})
	})

	// 修改学生信息接口
	r.PUT("/students/:id", func(c *gin.Context) {
		studentID, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid student ID"})
			return
		}
		var updates map[string]interface{}
		if err := c.ShouldBindJSON(&updates); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := studentManager.ModifyStudent(studentID, updates); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "Student information updated successfully"})
	})

	// 添加学生成绩接口
	r.POST("/students/:id/scores", func(c *gin.Context) {
		studentID, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid student ID"})
			return
		}
		var scoreData struct {
			CourseName string  `json:"course_name"`
			Score      float64 `json:"score"`
		}
		if err := c.ShouldBindJSON(&scoreData); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := studentManager.AddScore(studentID, scoreData.CourseName, scoreData.Score); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "Score added successfully"})
	})

	// 删除学生成绩接口
	r.DELETE("/students/:id/scores/:course", func(c *gin.Context) {
		studentID, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid student ID"})
			return
		}
		courseName := c.Param("course")
		if err := studentManager.DeleteScore(studentID, courseName); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "Score deleted successfully"})
	})

	// 修改学生成绩接口
	r.PUT("/students/:id/scores/:course", func(c *gin.Context) {
		studentID, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid student ID"})
			return
		}
		courseName := c.Param("course")
		var scoreData struct {
			Score float64 `json:"score"`
		}
		if err := c.ShouldBindJSON(&scoreData); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := studentManager.ModifyScore(studentID, courseName, scoreData.Score); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "Score updated successfully"})
	})

	// 查询学生信息接口
	r.GET("/students/:id", func(c *gin.Context) {
		studentID, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid student ID"})
			return
		}
		student, err := studentManager.QueryStudent(studentID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, student)
	})

	// 查询学生成绩接口
	r.GET("/students/:id/scores/:course", func(c *gin.Context) {
		studentID, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid student ID"})
			return
		}
		courseName := c.Param("course")
		score, err := studentManager.QueryScore(studentID, courseName)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"score": score})
	})

	// 启动服务
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	if err := r.Run(":" + port); err != nil {
		fmt.Println("Failed to start server:", err)
	}
}
