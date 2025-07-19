package synchrophasor

// MetricsRecorder is an interface for tracking various metrics related to client connections and data processing.
// RecordClientConnected logs a new client connection.
// RecordClientDisconnected logs a client disconnection.
// RecordCommand tracks the type of command being processed.
// RecordDataFrameSent tracks the size of data frames sent out.
// RecordConfigFrameSent tracks the size of configuration frames sent out.
// RecordHeaderFrameSent tracks the size of header frames sent out.
// RecordBytesReceived logs the size of data received.
// RecordFrameError tracks the type of frame error encountered.
// UpdateDataFrameRate updates the rate of data frame processing.
type MetricsRecorder interface {
	RecordClientConnected()
	RecordClientDisconnected()
	RecordCommand(cmdType string)
	RecordDataFrameSent(size int)
	RecordConfigFrameSent(size int)
	RecordHeaderFrameSent(size int)
	RecordBytesReceived(size int)
	RecordFrameError(errorType string)
	UpdateDataFrameRate(rate float64)
}
