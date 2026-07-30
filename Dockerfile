# Stage 1: Build Go server
FROM golang:1.26-bookworm AS go-builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY *.go ./
RUN CGO_ENABLED=0 go build -o esp32-meter-reader .

# Stage 2: Python + PaddleOCR
FROM python:3.13-slim-bookworm

RUN apt-get update && apt-get install -y --no-install-recommends \
    libgomp1 libglib2.0-0 libgl1 \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Create venv and install paddlepaddle + paddleocr                    
#                                                                                             
# Pin paddlepaddle to 3.2.2 — later 3.x versions have a CPU inference bug:                    
# https://github.com/PaddlePaddle/Paddle/issues/77340
# Fixed by https://github.com/PaddlePaddle/Paddle/pull/77430, but unreleased as
# of 3.2.2. Once a release carries that fix, all three pins below can move
# together (validate a rebuild against the previous versions before shipping).
#                                                                                             
# paddleocr/paddlex are pinned too: 3.7.x drives paddlepaddle 3.2.2 down a
# oneDNN kernel-selection path that writes "ReduceMeanCheckIfOneDNNSupport" to
# stdout, corrupting the JSON ocr.py prints there. 3.7.x also emits the meter's
# decimal separator, which OCR_MATCH_REGEX does not expect.                
RUN python3 -m venv /app/venv \                                                               
    && /app/venv/bin/pip install --no-cache-dir \
        paddlepaddle==3.2.2 \                                                                 
        -i https://www.paddlepaddle.org.cn/packages/stable/cpu/ \          
    && /app/venv/bin/pip install --no-cache-dir paddleocr                                     
    && /app/venv/bin/pip install --no-cache-dir \
        paddleocr==3.5.0 \                                                                    
        'paddlex[ocr-core]==3.5.2'       

# Pre-download models by doing a dummy inference
COPY ocr.py /app/ocr.py
RUN apt-get update && apt-get install -y --no-install-recommends wget \
    && wget -q -O /tmp/test.png https://paddle-model-ecology.bj.bcebos.com/paddlex/imgs/demo_image/general_ocr_002.png \
    && /app/venv/bin/python3 /app/ocr.py /tmp/test.png > /dev/null 2>&1 \
    && rm /tmp/test.png \
    && apt-get purge -y wget && apt-get autoremove -y \
    && rm -rf /var/lib/apt/lists/*

# Copy Go binary
COPY --from=go-builder /app/esp32-meter-reader /app/esp32-meter-reader

ENV PADDLE_PDX_DISABLE_MODEL_SOURCE_CHECK=True

EXPOSE 8080

CMD ["/app/esp32-meter-reader", \
     "--listen-addr", ":8080", \
     "--ocr-script", "/app/ocr.py", \
     "--python-bin", "/app/venv/bin/python3"]
