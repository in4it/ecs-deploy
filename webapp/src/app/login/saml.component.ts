
import {filter} from 'rxjs/operators';
import { Component, OnInit } from '@angular/core';

import { Router, ActivatedRoute } from '@angular/router';

import { AuthService } from '../services/index';



@Component({
  selector: 'app-saml',
  template: `<i class="fa fa-refresh fa-spin fa-3x fa-fw"></i><span class="sr-only">Loading...</span>`
})
export class LoginSAMLComponent implements OnInit {

  constructor(
   private authenticationService: AuthService,
   private route: ActivatedRoute,
   private router: Router) { }

  ngOnInit() {
    this.route.queryParams.pipe(
      filter(params => params.token))
      .subscribe(params => {
        if(params.token) {
          this.authenticationService.setToken(params.token)
          this.router.navigate(['services']);
        } else {
          this.router.navigate(['login']);
        }
    });
  }
}
