import { Injectable } from '@angular/core';
import { Router, NavigationStart } from '@angular/router';
import { Observable ,  Subject } from 'rxjs';

@Injectable()
export class AlertService {
    private subject = new Subject<any>();
    private keepAfterNavigationChange = false;

    constructor(private router: Router) {
        // clear alert message on route change
        router.events.subscribe(event => {
            if (event instanceof NavigationStart) {
                if (this.keepAfterNavigationChange) {
                    // only keep for a single location change
                    this.keepAfterNavigationChange = false;
                } else {
                    // clear alert
                    this.subject.next();
                }
            }
        });
    }

    success(message: string, removeAfterDelay = 0, keepAfterNavigationChange = false) {
        this.keepAfterNavigationChange = keepAfterNavigationChange;
        this.subject.next({ type: 'success', text: message });
        if(removeAfterDelay > 0 ) {
          setTimeout(()=>{ this.subject.next() }, removeAfterDelay * 1000)
        }
    }

    error(message: string, removeAfterDelay = 0, keepAfterNavigationChange = false) {
        this.keepAfterNavigationChange = keepAfterNavigationChange;
        this.subject.next({ type: 'error', text: message });
        if(removeAfterDelay > 0 ) {
          setTimeout(()=>{ this.subject.next() }, removeAfterDelay * 1000)
        }
    }

    getMessage(): Observable<any> {
        return this.subject.asObservable();
    }
}
