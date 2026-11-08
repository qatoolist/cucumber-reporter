Before { log('before hook') }
After { |s| attach('screenshot-bytes', 'image/png') if s.failed? }
AfterStep { }
