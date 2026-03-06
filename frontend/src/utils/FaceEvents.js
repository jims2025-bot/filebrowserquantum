/**
 * Global event bus for Face Recognition related events.
 * Used to coordinate updates between the Preview tab (face management)
 * and the Search sidebar (results and counts).
 */

const handlers = new Map();

export const FaceEvents = {
    emit: (event, data) => {
        if (!handlers.has(event)) return;
        handlers.get(event).forEach(handler => handler(data));
    },
    on: (event, handler) => {
        if (!handlers.has(event)) handlers.set(event, []);
        handlers.get(event).push(handler);
    },
    off: (event, handler) => {
        if (!handlers.has(event)) return;
        const index = handlers.get(event).indexOf(handler);
        if (index > -1) handlers.get(event).splice(index, 1);
    },
    
    // Event names
    FACE_UPDATED: 'face-updated'
};
