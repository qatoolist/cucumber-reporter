Before { log('before log') }
After { |s| attach(PNG, 'image/png') if s.failed?; attach('after text', 'text/plain') }
