Around('@around_fail') { |sc, blk| raise 'around boom before' }
Around('@around_after') { |sc, blk| blk.call; raise 'around boom after' }
