import pygame
import sys
from PIL import Image
import os
import numpy as np

# Initialize Pygame
pygame.init()

# Constants
SCALE_FACTOR = 2  # Character scale
FPS = 5  # Animation speed
WINDOW_WIDTH = 1024
WINDOW_HEIGHT = 768
TRANSPARENCY_THRESHOLD = 0  # Alpha threshold for transparency

# Set up the display
screen = pygame.display.set_mode((WINDOW_WIDTH, WINDOW_HEIGHT))
pygame.display.set_caption("Character Animation")

def ensure_directory(directory):
    if not os.path.exists(directory):
        os.makedirs(directory)

def make_transparent(image):
    # Convert image to RGBA if it isn't already
    if image.mode != 'RGBA':
        image = image.convert('RGBA')
    
    # Get the image data as a numpy array
    data = np.array(image)
    
    # Create a mask for white-ish pixels (RGB all close to 255)
    white_mask = (data[..., 0] > 250) & (data[..., 1] > 250) & (data[..., 2] > 250)
    
    # Set alpha to 0 for white pixels
    data[white_mask, 3] = 0
    
    # Create new image with the modified data
    return Image.fromarray(data)

def split_spritesheet(image_path):
    # Open the sprite sheet
    sprite_sheet = Image.open(image_path)
    
    # Get the dimensions of the sprite sheet
    width, height = sprite_sheet.size
    
    # Calculate individual sprite dimensions
    sprite_width = width // 4  # 4 frames per row
    sprite_height = height // 2  # 2 rows
    
    # Scale dimensions
    scaled_width = sprite_width * SCALE_FACTOR
    scaled_height = sprite_height * SCALE_FACTOR
    
    # Update window size if needed
    global WINDOW_WIDTH, WINDOW_HEIGHT
    WINDOW_WIDTH = max(WINDOW_WIDTH, scaled_width + 200)
    WINDOW_HEIGHT = max(WINDOW_HEIGHT, scaled_height + 200)
    pygame.display.set_mode((WINDOW_WIDTH, WINDOW_HEIGHT))
    
    # Lists to store frames
    idle_frames = []  # Top row - idle animation
    walk_frames = []  # Bottom row - walk animation
    
    # Extract idle animation frames (top row)
    for i in range(4):
        # Crop the frame
        frame = sprite_sheet.crop((i * sprite_width, 0, (i + 1) * sprite_width, sprite_height))
        # Make background transparent
        frame = make_transparent(frame)
        # Scale the frame
        frame = frame.resize((scaled_width, scaled_height), Image.Resampling.NEAREST)
        # Convert PIL image to Pygame surface with alpha
        frame_data = frame.tobytes()
        pygame_surface = pygame.image.fromstring(frame_data, frame.size, frame.mode)
        pygame_surface = pygame_surface.convert_alpha()
        idle_frames.append(pygame_surface)
    
    # Extract walking animation frames (bottom row)
    for i in range(4):
        # Crop the frame
        frame = sprite_sheet.crop((i * sprite_width, sprite_height, (i + 1) * sprite_width, height))
        # Make background transparent
        frame = make_transparent(frame)
        # Scale the frame
        frame = frame.resize((scaled_width, scaled_height), Image.Resampling.NEAREST)
        # Convert PIL image to Pygame surface with alpha
        frame_data = frame.tobytes()
        pygame_surface = pygame.image.fromstring(frame_data, frame.size, frame.mode)
        pygame_surface = pygame_surface.convert_alpha()
        walk_frames.append(pygame_surface)
    
    return walk_frames, idle_frames

def main():
    clock = pygame.time.Clock()
    
    # Load and split the sprite sheet
    walk_frames, idle_frames = split_spritesheet("rawImgs/images.png")
    
    current_frames = walk_frames  # Start with walking animation
    frame_index = 0
    is_walking = True
    
    # Game loop
    running = True
    while running:
        for event in pygame.event.get():
            if event.type == pygame.QUIT:
                running = False
            elif event.type == pygame.MOUSEBUTTONDOWN:
                is_walking = not is_walking
                current_frames = walk_frames if is_walking else idle_frames
                frame_index = 0
        
        # Clear the screen with light gray background for better visibility
        screen.fill((240, 240, 240))  # Light gray background
        
        # Draw the current frame
        current_frame = current_frames[frame_index]
        frame_rect = current_frame.get_rect(center=(WINDOW_WIDTH//2, WINDOW_HEIGHT//2 - 200))
        screen.blit(current_frame, frame_rect)
        
        # Update the frame index
        frame_index = (frame_index + 1) % len(current_frames)
        
        # Update the display
        pygame.display.flip()
        
        # Control the animation speed
        clock.tick(FPS)
    
    pygame.quit()
    sys.exit()

if __name__ == "__main__":
    main() 