# NeoBase Email Templates

This directory contains HTML email templates that follow the NeoBase design system and are optimized for email client compatibility.

## Design System

Our email templates follow the NeoBase neo-brutalism design system:

- **Colors**: 
  - Background: `#fef3c7` (yellow/20)
  - Primary: `#000000` (black)
  - Secondary: `#10b981` (green-500)
  - Accent: `#10b981` (green-500)
  - Text: `#374151` (gray-700)

- **Typography**: System fonts with fallbacks for email clients
- **Borders**: Bold 3px black borders
- **Shadows**: Neo-brutalism box shadows (6px 6px 0px) with green accent
- **Buttons**: Neo-brutalism style with green shadow hover effects
- **Logo**: Original NeoBase geometric logo with black strokes (no background)

## Template Structure

### Base Template (`base.html`)
Contains the common structure and styles used across all email templates.

### Individual Templates

#### `password_reset.html`
- **Purpose**: Password reset OTP emails
- **Placeholders**: 
  - `{{username}}` - User's display name
  - `{{otp}}` - 6-digit OTP code

#### `welcome.html`
- **Purpose**: Welcome emails for new users
- **Placeholders**:
  - `{{username}}` - User's display name

## Email Client Compatibility

Templates are optimized for:
- ✅ Gmail (Web, iOS, Android)
- ✅ Outlook (Web, Desktop, Mobile)
- ✅ Apple Mail (macOS, iOS)
- ✅ Yahoo Mail
- ✅ Thunderbird
- ✅ Mobile email clients

### Compatibility Features

1. **MSO Conditional Comments**: For Outlook desktop compatibility
2. **Table-based Layout**: Fallback for older email clients
3. **Inline CSS**: Critical styles inlined for better support
4. **Web Fonts with Fallbacks**: System fonts with graceful degradation
5. **Responsive Design**: Mobile-optimized with media queries
6. **Dark Mode Support**: Respects user preferences where supported

## Usage

Templates are loaded and processed by the `EmailService`:

```go
// Load template with placeholders
body, err := emailService.loadTemplate("password_reset", map[string]string{
    "username": "John Doe",
    "otp": "123456",
})
```

## Adding New Templates

1. Create a new `.html` file in this directory
2. Follow the existing design system and structure
3. Use `{{placeholder}}` syntax for dynamic content
4. Test across multiple email clients
5. Update the `EmailService` interface to include the new template method

## Testing

Before deploying new templates:

1. **Litmus/Email on Acid**: Test across email clients
2. **Mobile Testing**: Verify responsive behavior
3. **Accessibility**: Ensure proper contrast and structure
4. **Spam Testing**: Check spam score with template content

## Best Practices

1. **Keep it Simple**: Email clients have limited CSS support
2. **Inline Critical CSS**: Important styles should be inlined
3. **Use Tables**: For complex layouts, tables are more reliable
4. **Test Thoroughly**: Always test in real email clients
5. **Optimize Images**: Use web-optimized images with alt text
6. **Fallback Fonts**: Always provide font fallbacks
7. **Accessibility**: Use semantic HTML and proper contrast ratios