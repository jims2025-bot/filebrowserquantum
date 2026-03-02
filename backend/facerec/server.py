import os
import cv2
import numpy as np
from fastapi import FastAPI, UploadFile, File, HTTPException, Form
from fastapi.responses import JSONResponse
import uvicorn
import json

app = FastAPI(title="FileBrowser Quantum FaceRec Service (OpenCV YuNet/SFace)")

# Paths to models
YUNET_MODEL = os.path.join("models", "face_detection_yunet_2023mar.onnx")
SFACE_MODEL = os.path.join("models", "face_recognition_sface_2021dec.onnx")

# Initialize models
# YuNet (Detector)
detector = cv2.FaceDetectorYN.create(
    model=YUNET_MODEL,
    config="",
    input_size=(320, 320),
    score_threshold=0.85,
    nms_threshold=0.3,
    top_k=5000,
    backend_id=cv2.dnn.DNN_BACKEND_OPENCV,
    target_id=cv2.dnn.DNN_TARGET_CPU
)

# SFace (Recognizer/Embedder)
recognizer = cv2.FaceRecognizerSF.create(
    model=SFACE_MODEL,
    config="",
    backend_id=cv2.dnn.DNN_BACKEND_OPENCV,
    target_id=cv2.dnn.DNN_TARGET_CPU
)

def get_faces_yunet(img):
    h, w, _ = img.shape
    detector.setInputSize((w, h))
    _, faces = detector.detect(img)
    return faces if faces is not None else []

@app.post("/analyze")
async def analyze_image(file: UploadFile = File(...)):
    try:
        contents = await file.read()
        nparr = np.frombuffer(contents, np.uint8)
        img = cv2.imdecode(nparr, cv2.IMREAD_COLOR)
        if img is None:
            return JSONResponse(content={"faces": []})
            
        faces = get_faces_yunet(img)
        
        results = []
        for face in faces:
            # Face box is first 4 elements: x, y, w, h
            x, y, w, h = face[0:4].astype(int)
            # Correct to stay within image
            x1 = max(0, x)
            y1 = max(0, y)
            x2 = min(img.shape[1], x1 + w)
            y2 = min(img.shape[0], y1 + h)
            
            # Extract embedding (feature vector)
            aligned_face = recognizer.alignCrop(img, face)
            embedding = recognizer.feature(aligned_face)
            
            # Convert embedding to flat list for JSON
            emb_list = embedding.flatten().tolist()
            
            # Go backend expects [y1, x2, y2, x1]
            box = [int(y1), int(x2), int(y2), int(x1)]
            
            # Confidence is 15th element in YuNet face data
            score = float(face[14])
            
            results.append({
                "box": box,
                "name": "Unknown",
                "confidence": score,
                "embedding": emb_list
            })
            
        return JSONResponse(content={"faces": results})
    except Exception as e:
        print(f"Error analyzing image: {e}")
        return JSONResponse(content={"faces": [], "error": str(e)}, status_code=500)

@app.post("/learn")
async def learn_face(file: UploadFile = File(...), box: str = Form(...)):
    """
    Takes an image and a bounding box [y1, x2, y2, x1], 
    extracts the embedding for THAT specific region.
    """
    try:
        # box is passed as string '[y1, x2, y2, x1]'
        box_coords = json.loads(box)
        y1, x2, y2, x1 = box_coords
        
        contents = await file.read()
        nparr = np.frombuffer(contents, np.uint8)
        img = cv2.imdecode(nparr, cv2.IMREAD_COLOR)
        
        if img is None:
            raise HTTPException(status_code=400, detail="Invalid image")
            
        h_img, w_img, _ = img.shape
        # Clip to image bounds
        y1 = max(0, min(h_img, y1))
        y2 = max(0, min(h_img, y2))
        x1 = max(0, min(w_img, x1))
        x2 = max(0, min(w_img, x2))
        
        # We need a YuNet-style face array for alignCrop
        # Format: [x, y, w, h, x_re, y_re, x_le, y_le, x_nt, y_nt, x_rm, y_rm, x_lm, y_lm, score]
        # Since we don't have landmarks for manual boxes, we pass zeros.
        # recognizer.alignCrop works best WITH landmarks, but we can try just cropping manually if needed.
        # Actually, for manual boxes, it's better to just run the detector ON THE CROP to find landmarks if possible.
        
        crop = img[y1:y2, x1:x2]
        if crop.size == 0:
             raise HTTPException(status_code=400, detail="Empty crop region")
             
        # Run detector on just this crop to find landmarks
        faces = get_faces_yunet(crop)
        if len(faces) > 0:
            # Use the most central face detected in the crop
            # Correct landmarks back to original image coordinates
            face = faces[0]
            face[0] += x1 # x
            face[1] += y1 # y
            # landmarks
            for i in range(4, 14, 2):
                face[i] += x1
                face[i+1] += y1
            
            aligned_face = recognizer.alignCrop(img, face)
            embedding = recognizer.feature(aligned_face)
            emb_list = embedding.flatten().tolist()
            return JSONResponse(content={"embedding": emb_list, "success": True})
        else:
            # If no face found in crop via YuNet, just resize and get embedding as fallback
            # (Warning: this is less accurate but better than nothing)
            resized = cv2.resize(crop, (112, 112))
            # SFace expects a specific alignment. Passing raw crop is risky.
            # We'll just return error for now to encourage better boxes.
            return JSONResponse(content={"embedding": [], "success": False, "error": "No face detected in specified region"}, status_code=400)

    except Exception as e:
        print(f"Error in learn_face: {e}")
        return JSONResponse(content={"error": str(e)}, status_code=500)

if __name__ == "__main__":
    print(f"Starting FaceRec Server (YuNet/SFace) on port 8000...")
    uvicorn.run(app, host="127.0.0.1", port=8000)
