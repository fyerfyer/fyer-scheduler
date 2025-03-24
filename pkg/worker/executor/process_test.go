package executor

import (
    "context"
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

// TestProcessOutputCapture 测试进程输出捕获
func TestProcessOutputCapture(t *testing.T) {
    ctx := context.Background()
    
    // 用于存储捕获的输出
    var capturedOutput string
    
    // 创建输出处理函数
    outputHandler := func(output string) {
        t.Logf("Received output: %s", output)
        capturedOutput += output
    }
    
    // 测试Echo命令
    t.Run("EchoCommand", func(t *testing.T) {
        capturedOutput = "" // 重置输出
        
        options := ProcessOptions{
            ExecutionID:   "test-echo-output",
            Command:       "echo",
            Args:          []string{"Hello World"},
            OutputHandler: outputHandler,
        }
        
        proc, err := NewProcess(ctx, options)
        require.NoError(t, err, "Failed to create process")
        
        // 启动进程
        err = proc.Start()
        require.NoError(t, err, "Failed to start process")
        
        // 等待进程完成
        exitCode, err := proc.Wait()
        require.NoError(t, err, "Process wait failed")
        assert.Equal(t, 0, exitCode, "Echo command should exit with code 0")
        
        // 获取进程输出
        output := proc.GetOutput()
        t.Logf("Direct output: [%s]", output)
        t.Logf("Captured output: [%s]", capturedOutput)
        
        // 检查直接输出
        assert.Contains(t, output, "Hello World", "Direct output should contain the echoed text")
        
        // 检查通过handler捕获的输出
        assert.Contains(t, capturedOutput, "Hello World", "Captured output should contain the echoed text")
    })
    
    // 测试Uname命令
    t.Run("UnameCommand", func(t *testing.T) {
        capturedOutput = "" // 重置输出
        
        options := ProcessOptions{
            ExecutionID:   "test-uname-output",
            Command:       "uname",
            Args:          []string{"-a"},
            OutputHandler: outputHandler,
        }
        
        proc, err := NewProcess(ctx, options)
        require.NoError(t, err, "Failed to create process")
        
        // 启动进程
        err = proc.Start()
        require.NoError(t, err, "Failed to start process")
        
        // 等待进程完成
        exitCode, err := proc.Wait()
        require.NoError(t, err, "Process wait failed")
        assert.Equal(t, 0, exitCode, "Uname command should exit with code 0")
        
        // 获取进程输出
        output := proc.GetOutput()
        t.Logf("Direct output: [%s]", output)
        t.Logf("Captured output: [%s]", capturedOutput)
        
        // 检查输出不为空
        assert.NotEmpty(t, output, "Direct output should not be empty")
        assert.NotEmpty(t, capturedOutput, "Captured output should not be empty")
    })
}