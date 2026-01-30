  OpenRisk Dashboard Design - Visual Reference Card

 Color Palette Quick Reference

 Primary Colors


 Background      b (Deep Black)     
 Surface         b (Dark Navy)      
 Border          a (Subtle Gray)    
 Primary         bf (Bright Blue)     ← Main accent



 Risk Severity Colors


 Critical        ef (Red)             
 High            f (Orange)          
 Medium          eab (Yellow)          
 Low             bf (Blue)            



 Supporting Colors


 Success         b (Emerald)         
 Warning         feb (Amber)           
 Neutral         a (Zinc)            



---

 Widget Layout Grid


-Column Responsive Grid (Row Height: px)


                          HEADER                              
 Welcome back! | [Inventory] [Reset] [Export]                



  Risk Distribution        Risk Score Trends                  
  ( cols ×  rows)        ( cols ×  rows)                 
  - Donut Chart            - Line Chart                       
  -  segments             -  day history                   
  - Legend                 - Glowing dots                     
  - Summary Stats          - Smooth animations                



  Top Vulnerabilities      Avg Mitigation Time                
  ( cols ×  rows)        ( cols ×  rows)                 
  - Ranked list            - Semi-donut gauge                
  - Severity badges        - Time display                     
  - CVSS scores            - Completion stats                 
  - Asset counts           - Progress bar                     



                    Key Indicators                            
  ( cols ×  rows)                                          
  
   Critical      Total         Mitigated     Total        
   Risks         Risks         Risks         Assets       
                           /                
  



              Top Unmitigated Risks                           
  ( cols ×  rows)                                          
  .  Critical Vuln...              SCORE:  →            
  .  High Priority...              SCORE:  →            
  .  Service Issue...              SCORE:  →            
  .  Weak Control...               SCORE:   →            
  .  Encryption Gap...             SCORE:   →            



---

 Typography Reference


Page Title
 Font: Inter, px, Bold
 Color: White (ffffff)
 Gradient: from-white to-blue-
 Glow: text-shadow with s animation

Widget Title
 Font: Inter, px, Semibold
 Color: White (ffffff)
 Icon: Lucide icon (px) in primary blue
 Handle: Drag handle (px gray icon)

Stat Label
 Font: Inter, px, Regular
 Color: Zinc- (aaaa)
 Transform: Uppercase
 Tracking: Wider (.em)

Stat Value
 Font: Inter, px, Bold
 Color: White (ffffff)
 Style: Monospace for numbers

Badge Text
 Font: Inter, px, Bold
 Color: Severity-based
 Background: Semi-transparent color
 Border: Matching color (. opacity)

Tooltip Text
 Font: Inter, px, Regular
 Color: Zinc- (b)
 Background: Dark blue (b)
 Border: Subtle gray (a)


---

 Component Sizing Guide

 Widget Cards

Desktop (px+):
 Half-width ( cols): % - px margin
 Full-width ( cols): % - px padding
 Height:  rows = px + px margin
 Border radius: px (rounded-xl)
 Shadows: shadow-xl with glow

Tablet (px):
 Half-width stacks to full width
 Cards resize to available space
 Minimum width: px
 Touch-friendly padding: px

Mobile (px):
 Full-width cards
 Padding: px sides
 Height: Auto or px
 List items: - visible before scroll


 Icon Sizing

Widget Title Icon:     px (primary blue)
Drag Handle:          px (zinc-)
Severity Icons:       px (color-coded)
Stat Icons:           px (in cards)
Badge Icons:          px (in badges)
Chevron Icons:        px (subtle)


 Spacing System

Base Unit: px (Tailwind scale)

Padding:
  sm: px (p-)
  md: px (p-)
  lg: px (p-)
  xl: px (p-)

Gaps:
  sm: px (gap-)
  md: px (gap-)
  lg: px (gap-)
  xl: px (gap-)

Margins:
  Widget margin: px (between cards)
  Item margin: px (between list items)


---

 Animation Reference

 Fade In

Duration: .s
Easing: ease-out
From: opacity-, translateY(px)
To: opacity-, translateY()


 Glow Pulse

Duration: s
Easing: ease-in-out
Infinite: Yes
Effect: Box shadow pulsing
Colors: blue (% to % opacity)


 Neon Glow (Text)

Duration: s
Easing: ease-in-out
Infinite: Yes
Effect: Text shadow flickering
Colors: blue text shadow


 Hover Scale

Duration: ms
Easing: ease
From: scale-
To: scale- (% increase)


 Grid Transitions

Duration: ms
Easing: ease
Properties: left, top, width, height


---

 Glassmorphism Effect Breakdown


Step : Background
  Linear Gradient: deg
  From: rgba(, , , .) %
  To: rgba(, , , ) %

Step : Backdrop
  Filter: blur(px)
  -webkit-Filter: blur(px) (Safari)

Step : Border
  Color: rgba(, , , .)
  Width: px
  Radius: px

Step : Shadow
  Box-shadow:  px px rgba(, , , .)

Step : Hover Effect
  Background: rgba(, , , .) %, rgba(, , , .) %
  Border: rgba(, , , .) (blue tint)
  Shadow:  px px rgba(, , , .) (blue glow)
  Duration: ms smooth transition


---

 Neon Glow Reference

 Box Glow

Primary (Blue):
  box-shadow:   px rgba(, , , .)

Critical (Red):
  box-shadow:   px rgba(, , , .)

High (Orange):
  box-shadow:   px rgba(, , , .)

Large Glow:
  box-shadow:   px rgba(, , , .)


 Text Glow

Base Effect:
  text-shadow:   px rgba(, , , .),
                 px rgba(, , , .)

Animated Effect (% in animation):
  text-shadow:   px rgba(, , , .),
                 px rgba(, , , .)


---

 Responsive Breakpoints


Mobile Small (px)
 Single column layout
 Cards: % width - px padding
 Lists: - items visible
 Stat cards: × grid

Mobile Medium (px)
 Single to dual column
 Cards: % width
 Improved spacing
 Better touch targets

Tablet (px)
  columns
 Side-by-side widgets
 Horizontal lists
 Stat cards: × or ×

Desktop (px+)
 Full -column grid
 Drag-and-drop enabled
 Side navigation visible
 All features accessible


---

 State Variations

 Widget States

Default State:

Border: rgba(, , , .)
Background: linear-gradient(from-white/ to-white/)
Shadow: shadow-xl


Hover State:

Border: rgba(, , , .) ← Blue tint
Background: linear-gradient(from-white/ to-white/) ← Brighter
Shadow: shadow-xl with blue glow
Duration: ms transition


Dragging State:

Opacity: . (semi-transparent)
Border: Same
Shadow: Lighter
Cursor: grabbing


Loading State:

Content: Spinner animation
Overlay: Semi-transparent
Message: "Loading..."


 Badge States

Default:

Background: Severity-based color (% opacity)
Border: Severity-based color (% opacity)
Text: Severity-based color (% opacity)


Hover (on parent):

Background: Brighter (% opacity)
Border: Brighter (% opacity)
Glow: Severity-based box-shadow
Scale: %


---

 Accessibility Checklist


Color Contrast:
 Text on background: :+ ratio (AAA)
 Icons on background: .:+ ratio (AA)
 Badges readable: High contrast maintained

Focus States:
 All interactive elements: Visible focus ring
 Keyboard navigation: Tab order logical
 Focus outline: px solid primary blue

Labels & Text:
 Icon + text labels: Combined clarity
 ARIA labels: On interactive elements
 Semantic HTML: Proper heading hierarchy

Motion:
 Reduced motion: Respects prefers-reduced-motion
 Animation speed: Not too fast or jarring
 Flashing: No content flashes >  times/sec


---

 Quick Copy-Paste Classes

 Text Effects

gradient-text: bg-gradient-to-r from-white to-blue- bg-clip-text text-transparent
neon-glow: animate-neon-glow
glow-pulse: animate-glow-pulse
fade-in: animate-fade-in


 Widget Effects

widget-glass: rounded-xl border border-white/ bg-gradient-to-br from-white/ to-white/ backdrop-blur-xl shadow-xl
hover-glass: hover:border-white/ hover:bg-gradient-to-br hover:from-white/ hover:to-white/ hover:shadow-glow-lg transition-all duration-
badge-glow: badge-glow-critical (for critical severity)
neon-glow: box-shadow:   px rgba(, , , .)


 Responsive Helpers

mobile-only: sm:hidden
desktop-only: hidden md:block
full-width: w-full
half-width: w-/ md:w-full


---

Version: .  
Last Updated: January ,   
Status:  Complete Reference
