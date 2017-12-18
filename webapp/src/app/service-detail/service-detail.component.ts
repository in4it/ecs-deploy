import { Component, OnInit } from '@angular/core';
import { ActivatedRoute, Router } from '@angular/router';
import { Observable } from 'rxjs/Observable';

import { ServiceDetail, ServiceDetailService }  from './service-detail.service';

@Component({
  selector: 'app-service-detail',
  templateUrl: './service-detail.component.html',
  styleUrls: ['./service-detail.component.css']
})
export class ServiceDetailComponent implements OnInit {

  service: any = {};

  tab = "service"

  constructor(
    private route: ActivatedRoute,
    private router: Router,
    private sds: ServiceDetailService
  ) {}

  ngOnInit(): void {
    this.route.data
      .subscribe((data: { sd: ServiceDetail }) => {
        this.service = data.sd.service
     });
  }

  onClickVersions() {
    this.tab = "versions"
  }
  onClickService() {
    this.tab = "service"
  }

}
