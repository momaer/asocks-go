package asocks

var bufferPool = make(chan []byte, 100)    

func GetBuffer() []byte {
    var buffer []byte
    select {
        case buffer = <-bufferPool:
        default:
            buffer = make([]byte, 5120)
    }
    return buffer
}

func GiveBuffer(buffer []byte) {
    select {
        case bufferPool <- buffer:
        default:
    }
}
