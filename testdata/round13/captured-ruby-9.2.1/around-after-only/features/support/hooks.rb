Around('@around_after') { |sc, blk| blk.call; raise 'around boom after' }
