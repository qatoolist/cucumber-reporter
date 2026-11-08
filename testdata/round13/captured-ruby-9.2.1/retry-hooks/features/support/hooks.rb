require 'base64'
PNG = Base64.decode64('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==')
Before('@before_fail') { log 'before log before raise'; raise 'before boom' }
After('@after_fail') { attach('after text evidence', 'text/plain'); raise 'after boom' }
AfterStep('@afterstep_fail') { raise 'afterstep boom' }
Around('@around_fail') { |sc, blk| raise 'around boom before' }
Around('@around_after') { |sc, blk| blk.call; raise 'around boom after' }
Before('@output') { log 'before hook log'; attach('before plain', 'text/plain'); attach({a: 1}.to_json, 'application/json') }
After('@output') { attach(PNG, 'image/png'); attach('<b>html from after</b>', 'text/html'); log 'after hook log' }
AfterStep('@output') { attach('afterstep text', 'text/plain') }
