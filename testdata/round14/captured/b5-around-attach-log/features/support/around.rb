Around('@ar') do |scenario, block|
  log "around-before-log #{scenario.name}"
  attach "around-before-attach #{scenario.name}", 'text/plain'
  block.call
  log "around-after-log #{scenario.name}"
  attach "around-after-attach #{scenario.name}", 'text/plain'
end
