import sys
from PIL import Image
import numpy as np
import cv2
import os

# Constants
SCALE_FACTOR = 3  # Scale up the sprites for better visibility
SPRITE_ROWS = 4
SPRITE_COLS = 3
GIF_DURATION = 200  # Duration for each frame in milliseconds (slowed down)

# Color settings
BG_COLOR = (46, 34, 47)  # Dark brown background color
SKIN_COLOR = (255, 206, 177)  # Approximate skin tone color
COLOR_TOLERANCE = 30     # Color tolerance for background removal
SKIN_TOLERANCE = 80     # Higher tolerance for skin tone variations

def make_transparent(image):
    """Make background transparent while preserving character sprites including skin tones."""
    if image.mode != 'RGBA':
        image = image.convert('RGBA')
    
    data = np.array(image)
    
    # Calculate color distances
    bg_r_diff = np.abs(data[..., 0] - BG_COLOR[0])
    bg_g_diff = np.abs(data[..., 1] - BG_COLOR[1])
    bg_b_diff = np.abs(data[..., 2] - BG_COLOR[2])
    bg_distance = np.sqrt(bg_r_diff**2 + bg_g_diff**2 + bg_b_diff**2)
    
    skin_r_diff = np.abs(data[..., 0] - SKIN_COLOR[0])
    skin_g_diff = np.abs(data[..., 1] - SKIN_COLOR[1])
    skin_b_diff = np.abs(data[..., 2] - SKIN_COLOR[2])
    skin_distance = np.sqrt(skin_r_diff**2 + skin_g_diff**2 + skin_b_diff**2)
    
    # Create masks
    is_background = bg_distance < COLOR_TOLERANCE
    is_skin = skin_distance < SKIN_TOLERANCE
    
    # Create edge detection mask
    gray = cv2.cvtColor(data, cv2.COLOR_RGBA2GRAY)
    edges = cv2.Canny(gray, 50, 150)
    has_edge = edges > 0
    
    # Preserve pixels that are either skin tone or near edges
    keep_mask = is_skin | has_edge | ~is_background
    
    # Apply transparency
    data[~keep_mask, 3] = 0
    
    return Image.fromarray(data)

def split_spritesheet(image_path):
    """Split the spritesheet into individual frames."""
    try:
        sprite_sheet = Image.open(image_path)
    except FileNotFoundError:
        print(f"Error: Could not find sprite sheet at {image_path}")
        sys.exit(1)
    
    # Get dimensions
    width, height = sprite_sheet.size
    sprite_width = width // SPRITE_COLS
    sprite_height = height // SPRITE_ROWS
    
    # Scale dimensions
    scaled_width = sprite_width * SCALE_FACTOR
    scaled_height = sprite_height * SCALE_FACTOR
    
    frames = []
    
    # Create output directory if it doesn't exist
    os.makedirs("./imgs/rawImgs/frames", exist_ok=True)
    
    # Extract frames
    for row in range(SPRITE_ROWS):
        for col in range(SPRITE_COLS):
            # Calculate frame boundaries
            x1 = col * sprite_width
            y1 = row * sprite_height
            x2 = x1 + sprite_width
            y2 = y1 + sprite_height
            
            # Crop the frame
            frame = sprite_sheet.crop((x1, y1, x2, y2))
            
            # Make background transparent while preserving character
            frame = make_transparent(frame)
            
            # Scale the frame
            frame = frame.resize((scaled_width, scaled_height), Image.Resampling.NEAREST)
            
            # Save individual frame
            frame.save(f"./imgs/rawImgs/frames/frame_{row}_{col}.png")
            
            # Add frame to list
            frames.append(frame)
    
    return frames

def create_gif(frames, output_path, duration=GIF_DURATION):
    """Create a GIF from the frames."""
    # Save the first frame
    frames[0].save(
        output_path,
        save_all=True,
        append_images=frames[1:],
        duration=duration,
        loop=0,
        optimize=False
    )

def main():
    # Load and split the sprite sheet
    frames = split_spritesheet("./imgs/rawImgs/images3.png")
    
    # Create GIF
    create_gif(frames, "./imgs/rawImgs/animation.gif")
    print("Animation saved as animation.gif")

if __name__ == "__main__":
    main() 