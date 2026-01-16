/**
 * Notification sound utility using Web Audio API
 * Generates a pleasant two-tone chime for order event notifications
 */

let audioContext = null;

function getAudioContext() {
    if (!audioContext) {
        audioContext = new (window.AudioContext || window.webkitAudioContext)();
    }
    return audioContext;
}

/**
 * Play a pleasant two-tone notification chime
 * Sound design: C5 (523Hz) -> E5 (659Hz) with soft envelope
 */
export function playOrderNotificationSound() {
    try {
        const ctx = getAudioContext();
        
        // Resume context if suspended (browser autoplay policy)
        if (ctx.state === 'suspended') {
            ctx.resume();
        }
        
        const now = ctx.currentTime;
        const volume = 0.15; // Soft volume (15%)
        
        // Create master gain for overall volume
        const masterGain = ctx.createGain();
        masterGain.connect(ctx.destination);
        masterGain.gain.value = volume;
        
        // First tone: C5 (523 Hz)
        playTone(ctx, masterGain, 523, now, 0.12);
        
        // Second tone: E5 (659 Hz) - slightly delayed
        playTone(ctx, masterGain, 659, now + 0.1, 0.15);
        
    } catch (error) {
        // Silently fail - audio is non-critical
        console.debug('Could not play notification sound:', error.message);
    }
}

/**
 * Play a single tone with smooth envelope
 */
function playTone(ctx, destination, frequency, startTime, duration) {
    // Oscillator for the tone
    const oscillator = ctx.createOscillator();
    oscillator.type = 'sine';
    oscillator.frequency.value = frequency;
    
    // Gain envelope for smooth attack/decay
    const gainNode = ctx.createGain();
    gainNode.connect(destination);
    
    // Envelope: quick attack, smooth decay
    gainNode.gain.setValueAtTime(0, startTime);
    gainNode.gain.linearRampToValueAtTime(1, startTime + 0.02); // 20ms attack
    gainNode.gain.exponentialRampToValueAtTime(0.01, startTime + duration); // decay
    
    oscillator.connect(gainNode);
    oscillator.start(startTime);
    oscillator.stop(startTime + duration + 0.05);
}

export default {
    playOrderNotificationSound
};
